package git

import (
	"errors"
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
