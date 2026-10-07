package git

import (
	"reflect"
	"testing"
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
