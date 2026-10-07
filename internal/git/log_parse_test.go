package git

import (
	"errors"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"
)

// Decoration fixtures are the exact %D strings observed under
// `git log --decorate=short` (issue #23), including the "HEAD -> branch",
// detached "HEAD, branch" and "tag: name" grammars.

func TestParseRefs(t *testing.T) {
	cases := []struct {
		in   string
		want []Ref
	}{
		{"", []Ref{}},
		{"HEAD -> main", []Ref{{Kind: RefHead, Name: "main"}}},
		{"HEAD, main", []Ref{{Kind: RefHead, Name: "HEAD"}, {Kind: RefBranch, Name: "main"}}},
		{"tag: v1.0, tag: annotated, feat", []Ref{
			{Kind: RefTag, Name: "v1.0"},
			{Kind: RefTag, Name: "annotated"},
			{Kind: RefBranch, Name: "feat"},
		}},
		{"origin/main", []Ref{{Kind: RefRemote, Name: "origin/main"}}},
		{"HEAD -> main, origin/main, origin/HEAD, tag: v0.1", []Ref{
			{Kind: RefHead, Name: "main"},
			{Kind: RefRemote, Name: "origin/main"},
			{Kind: RefRemote, Name: "origin/HEAD"},
			{Kind: RefTag, Name: "v0.1"},
		}},
		// comma inside a ref name survives: names contain no spaces, so
		// only ", " separates decorations
		{"tag: an,tag", []Ref{{Kind: RefTag, Name: "an,tag"}}},
	}
	for _, tc := range cases {
		got := parseRefs(tc.in)
		if !reflect.DeepEqual(got, tc.want) {
			t.Fatalf("parseRefs(%q) = %+v, want %+v", tc.in, got, tc.want)
		}
	}
}

func logRec(fields ...string) string {
	return "\x1e" + strings.Join(fields, "\x1f") + "\n"
}

func TestParseLogOutput(t *testing.T) {
	root := logRec(
		"aaa111", "aaa", "", "Test User", "t@t", "1700000000", "1700000100",
		"initial commit", "body line one\nbody line two\n", "HEAD -> main",
	)
	second := logRec(
		"bbb222", "bbb", "aaa111", "Test User", "t@t", "1700000200", "1700000200",
		"second commit", "", "tag: v1.0, origin/main",
	)
	got, err := parseLogOutput([]byte(root + second))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	want := []Commit{
		{
			Hash: "aaa111", ShortHash: "aaa", ParentHashes: []string{},
			AuthorName: "Test User", AuthorEmail: "t@t",
			AuthorDate:    time.Unix(1700000000, 0).UTC(),
			CommitterDate: time.Unix(1700000100, 0).UTC(),
			Subject:       "initial commit",
			Body:          "body line one\nbody line two",
			Refs:          []Ref{{Kind: RefHead, Name: "main"}},
		},
		{
			Hash: "bbb222", ShortHash: "bbb", ParentHashes: []string{"aaa111"},
			AuthorName: "Test User", AuthorEmail: "t@t",
			AuthorDate:    time.Unix(1700000200, 0).UTC(),
			CommitterDate: time.Unix(1700000200, 0).UTC(),
			Subject:       "second commit",
			Body:          "",
			Refs: []Ref{
				{Kind: RefTag, Name: "v1.0"},
				{Kind: RefRemote, Name: "origin/main"},
			},
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got  %+v\nwant %+v", got, want)
	}
}

func TestParseLogOutputBodyWithSeparatorBytes(t *testing.T) {
	// \x1f can be committed in a message verbatim; the record is framed so
	// the decoration field (which can never contain \x1f) is split from
	// the right, keeping the body intact
	rec := logRec("c1", "c", "", "An", "ae", "1", "2", "subj", "weird \x1f body \x1f\n", "main")
	got, err := parseLogOutput([]byte(rec))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if got[0].Body != "weird \x1f body \x1f" {
		t.Fatalf("body = %q, want control bytes preserved", got[0].Body)
	}
	if got[0].Subject != "subj" || got[0].Refs[0].Name != "main" {
		t.Fatalf("record misframed: %+v", got[0])
	}
}

func TestParseLogOutputMalformed(t *testing.T) {
	cases := map[string]string{
		"too few fields":        "\x1ec1\x1fshort\x1fbody\n",
		"missing refs boundary": "\x1ec1\x1fch\x1f\x1fan\x1fae\x1f1\x1f2\x1fs\x1fbody-without-sep",
		"bad epoch":             logRec("c1", "c", "", "An", "ae", "not-a-number", "2", "s", "", ""),
		"bad committer epoch":   logRec("c1", "c", "", "An", "ae", "1", "x", "s", "", ""),
		"huge malformed record": "\x1e" + strings.Repeat("z", 300) + "\n",
	}
	for name, in := range cases {
		if _, err := parseLogOutput([]byte(in)); !errors.Is(err, ErrParseFailed) {
			t.Fatalf("%s: err = %v, want parse_failed", name, err)
		}
	}
}

func TestParseLogOutputEmpty(t *testing.T) {
	got, err := parseLogOutput(nil)
	if err != nil || len(got) != 0 {
		t.Fatalf("got %+v err %v, want empty", got, err)
	}
}

// TestParseLogOutputFixtures round-trips issue #23's acceptance case:
// testdata/log_linear.log and log_merges.log captured verbatim from
// `git log --decorate=short --format=<logFormat>` over real repos (tag +
// fake origin ref for decorations, unicode multi-line body, merge,
// octopus and empty commits). Expectations are hand-transcribed from the
// fixture bytes.
func TestParseLogOutputFixtures(t *testing.T) {
	cases := []struct {
		file string
		want []Commit
	}{
		{"log_linear.log", []Commit{
			{
				Hash: "3b94f0f874a7a79b3d1fdbc3ba540f8ee83854db", ShortHash: "3b94f0f",
				ParentHashes:  []string{"1459c2170e3c4d8e95c47e157cd659eeeb3c33f6"},
				AuthorName:    "test",
				AuthorEmail:   "t@t",
				AuthorDate:    time.Unix(1768221000, 0).UTC(),
				CommitterDate: time.Unix(1768221060, 0).UTC(),
				Subject:       "third commit",
				Body:          "",
				Refs:          []Ref{{Kind: RefHead, Name: "main"}, {Kind: RefRemote, Name: "origin/main"}},
			},
			{
				Hash: "1459c2170e3c4d8e95c47e157cd659eeeb3c33f6", ShortHash: "1459c21",
				ParentHashes:  []string{"a87fb1c516caf09108aaad53ca3914d2f13be00b"},
				AuthorName:    "test",
				AuthorEmail:   "t@t",
				AuthorDate:    time.Unix(1768125600, 0).UTC(),
				CommitterDate: time.Unix(1768125600, 0).UTC(),
				Subject:       "second commit",
				Body:          "",
				Refs:          []Ref{},
			},
			{
				Hash:          "a87fb1c516caf09108aaad53ca3914d2f13be00b",
				ShortHash:     "a87fb1c",
				ParentHashes:  []string{},
				AuthorName:    "test",
				AuthorEmail:   "t@t",
				AuthorDate:    time.Unix(1768032000, 0).UTC(),
				CommitterDate: time.Unix(1768032300, 0).UTC(),
				Subject:       "initial commit ✨",
				Body:          "multi\nline body\nzażółta gęśl, jaźń",
				Refs:          []Ref{{Kind: RefTag, Name: "v0.9"}},
			},
		}},
		{"log_merges.log", []Commit{
			{
				Hash: "4553fdd71a4a592a32dd44375827600cdcb20cca", ShortHash: "4553fdd",
				ParentHashes: []string{
					"a84b1bee224e83b6286dde845d3da3a15855ebe2",
					"a5f7109a90c3fba469aa303eb2b057266517c771",
					"3fb269dbc57ced6a3ee99bf6fe25660741bca4eb",
				},
				AuthorName: "test", AuthorEmail: "t@t",
				AuthorDate: time.Unix(1791377697, 0).UTC(), CommitterDate: time.Unix(1791377697, 0).UTC(),
				Subject: "octopus", Body: "",
				Refs: []Ref{{Kind: RefHead, Name: "main"}},
			},
			{
				Hash: "a84b1bee224e83b6286dde845d3da3a15855ebe2", ShortHash: "a84b1be",
				ParentHashes: []string{
					"b240ac73abf68ede276bbedd3c9bb32a2095228e",
					"ee810c5b86dababa63a602f3606098c3dcdfe6ac",
				},
				AuthorName: "test", AuthorEmail: "t@t",
				AuthorDate: time.Unix(1791377697, 0).UTC(), CommitterDate: time.Unix(1791377697, 0).UTC(),
				Subject: "two-parent", Body: "", Refs: []Ref{},
			},
			{
				Hash: "b240ac73abf68ede276bbedd3c9bb32a2095228e", ShortHash: "b240ac7",
				ParentHashes:  []string{"b00d58f1272b9dfd038de9cb719cd9f6f3982706"},
				AuthorName:    "test",
				AuthorEmail:   "t@t",
				AuthorDate:    time.Unix(1769925600, 0).UTC(),
				CommitterDate: time.Unix(1769925600, 0).UTC(),
				Subject:       "empty commit", Body: "", Refs: []Ref{},
			},
			{
				Hash: "3fb269dbc57ced6a3ee99bf6fe25660741bca4eb", ShortHash: "3fb269d",
				ParentHashes:  []string{"b00d58f1272b9dfd038de9cb719cd9f6f3982706"},
				AuthorName:    "test",
				AuthorEmail:   "t@t",
				AuthorDate:    time.Unix(1769914800, 0).UTC(),
				CommitterDate: time.Unix(1769914800, 0).UTC(),
				Subject:       "side 3", Body: "",
				Refs: []Ref{{Kind: RefBranch, Name: "s3"}},
			},
			{
				Hash: "a5f7109a90c3fba469aa303eb2b057266517c771", ShortHash: "a5f7109",
				ParentHashes:  []string{"b00d58f1272b9dfd038de9cb719cd9f6f3982706"},
				AuthorName:    "test",
				AuthorEmail:   "t@t",
				AuthorDate:    time.Unix(1769911200, 0).UTC(),
				CommitterDate: time.Unix(1769911200, 0).UTC(),
				Subject:       "side 2", Body: "",
				Refs: []Ref{{Kind: RefBranch, Name: "s2"}},
			},
			{
				Hash: "ee810c5b86dababa63a602f3606098c3dcdfe6ac", ShortHash: "ee810c5",
				ParentHashes:  []string{"b00d58f1272b9dfd038de9cb719cd9f6f3982706"},
				AuthorName:    "test",
				AuthorEmail:   "t@t",
				AuthorDate:    time.Unix(1769907600, 0).UTC(),
				CommitterDate: time.Unix(1769907600, 0).UTC(),
				Subject:       "side 1", Body: "",
				Refs: []Ref{{Kind: RefBranch, Name: "s1"}},
			},
			{
				Hash: "b00d58f1272b9dfd038de9cb719cd9f6f3982706", ShortHash: "b00d58f",
				ParentHashes:  []string{},
				AuthorName:    "test",
				AuthorEmail:   "t@t",
				AuthorDate:    time.Unix(1769904000, 0).UTC(),
				CommitterDate: time.Unix(1769904000, 0).UTC(),
				Subject:       "base", Body: "", Refs: []Ref{},
			},
		}},
	}
	for _, tc := range cases {
		t.Run(tc.file, func(t *testing.T) {
			data, err := os.ReadFile("testdata/" + tc.file)
			if err != nil {
				t.Fatal(err)
			}
			got, err := parseLogOutput(data)
			if err != nil {
				t.Fatalf("parse %s: %v", tc.file, err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("got  %+v\nwant %+v", got, tc.want)
			}
		})
	}
}
