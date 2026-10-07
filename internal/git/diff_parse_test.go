package git

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

func diffJoin(lines ...string) []byte {
	return []byte(strings.Join(lines, "\n") + "\n")
}

// Fixtures use the record shapes from git's unified diff format, verified
// against real `git diff` output (issue #24). Expected line numbers are
// worked out by hand from each fixture, not recomputed from the parser.

func TestParseUnifiedDiffModify(t *testing.T) {
	in := diffJoin(
		"diff --git a/a.txt b/a.txt",
		"index c9e9e05..bcb899d 100755",
		"--- a/a.txt",
		"+++ b/a.txt",
		"@@ -1,10 +1,10 @@",
		" one",
		"-two",
		"+TWO",
		" three",
		" four",
		" five",
		" six",
		" seven",
		"-eight",
		"+sevenb",
		" nine",
		" ten",
	)
	got, err := parseUnifiedDiff(in)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	want := []FileDiff{{
		OldPath: "a.txt",
		NewPath: "a.txt",
		Change:  ChangeModified,
		Hunks: []DiffHunk{{
			Header:   "@@ -1,10 +1,10 @@",
			OldStart: 1, OldCount: 10, NewStart: 1, NewCount: 10,
			Lines: []DiffLine{
				{Type: DiffLineContext, OldNum: 1, NewNum: 1, Text: "one"},
				{Type: DiffLineDel, OldNum: 2, Text: "two"},
				{Type: DiffLineAdd, NewNum: 2, Text: "TWO"},
				{Type: DiffLineContext, OldNum: 3, NewNum: 3, Text: "three"},
				{Type: DiffLineContext, OldNum: 4, NewNum: 4, Text: "four"},
				{Type: DiffLineContext, OldNum: 5, NewNum: 5, Text: "five"},
				{Type: DiffLineContext, OldNum: 6, NewNum: 6, Text: "six"},
				{Type: DiffLineContext, OldNum: 7, NewNum: 7, Text: "seven"},
				{Type: DiffLineDel, OldNum: 8, Text: "eight"},
				{Type: DiffLineAdd, NewNum: 8, Text: "sevenb"},
				{Type: DiffLineContext, OldNum: 9, NewNum: 9, Text: "nine"},
				{Type: DiffLineContext, OldNum: 10, NewNum: 10, Text: "ten"},
			},
		}},
	}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parse mismatch\ngot:  %+v\nwant: %+v", got, want)
	}
}

func TestParseUnifiedDiffMultipleHunks(t *testing.T) {
	in := diffJoin(
		"diff --git a/many.txt b/many.txt",
		"index 97b3d1a..2692797 100644",
		"--- a/many.txt",
		"+++ b/many.txt",
		"@@ -1,4 +1,4 @@",
		"-1",
		"+X",
		" 2",
		" 3",
		" 4",
		"@@ -12,4 +12,4 @@",
		" 12",
		" 13",
		" 14",
		"-15",
		"+Y",
	)
	got, err := parseUnifiedDiff(in)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(got) != 1 || len(got[0].Hunks) != 2 {
		t.Fatalf("got %+v, want one file with two hunks", got)
	}
	h := got[0].Hunks[1]
	if h.OldStart != 12 || h.OldCount != 4 || h.NewStart != 12 || h.NewCount != 4 {
		t.Fatalf("second hunk range = %+v, want -12,4 +12,4", h)
	}
	if h.Lines[0] != (DiffLine{Type: DiffLineContext, OldNum: 12, NewNum: 12, Text: "12"}) {
		t.Fatalf("second hunk first line = %+v, want ctx 12/12 \"12\"", h.Lines[0])
	}
	if h.Lines[4] != (DiffLine{Type: DiffLineAdd, NewNum: 15, Text: "Y"}) {
		t.Fatalf("second hunk last line = %+v, want add new 15 \"Y\"", h.Lines[4])
	}
}

func TestParseUnifiedDiffOmittedCounts(t *testing.T) {
	// real `git diff -U0` shape: a count of 1 is omitted from @@ ranges
	in := diffJoin(
		"diff --git a/many.txt b/many.txt",
		"--- a/many.txt",
		"+++ b/many.txt",
		"@@ -1 +1 @@",
		"-1",
		"+X",
		"@@ -15 +15 @@",
		"-15",
		"+Y",
	)
	got, err := parseUnifiedDiff(in)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	h := got[0].Hunks[0]
	if h.OldStart != 1 || h.OldCount != 1 || h.NewStart != 1 || h.NewCount != 1 {
		t.Fatalf("hunk = %+v, want -1,1 +1,1", h)
	}
}

func TestParseUnifiedDiffNoNewline(t *testing.T) {
	in := diffJoin(
		"diff --git a/n.txt b/n.txt",
		"--- a/n.txt",
		"+++ b/n.txt",
		"@@ -1,2 +1,2 @@",
		" a",
		"-b",
		"+c",
		`\ No newline at end of file`,
	)
	got, err := parseUnifiedDiff(in)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	lines := got[0].Hunks[0].Lines
	if !lines[2].NoNewline {
		t.Fatalf("added line %+v must carry the no-newline flag", lines[2])
	}
	if len(lines) != 3 {
		t.Fatalf("got %d lines, want 3: the marker is not its own line", len(lines))
	}
}

func TestParseUnifiedDiffQuotedPaths(t *testing.T) {
	in := diffJoin(
		`diff --git "a/wei\"rd.txt" "b/wei\"rd.txt"`,
		`--- "a/wei\"rd.txt"`,
		`+++ "b/wei\"rd.txt"`,
		"@@ -1 +1 @@",
		"-q",
		"+q2",
	)
	got, err := parseUnifiedDiff(in)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if got[0].OldPath != `wei"rd.txt` || got[0].NewPath != `wei"rd.txt` {
		t.Fatalf("paths = %q %q, want wei\"rd.txt", got[0].OldPath, got[0].NewPath)
	}
}

func TestParseUnifiedDiffMultipleFiles(t *testing.T) {
	in := diffJoin(
		"diff --git a/x.txt b/x.txt",
		"--- a/x.txt",
		"+++ b/x.txt",
		"@@ -1 +1 @@",
		"-a",
		"+b",
		"diff --git a/y.txt b/y.txt",
		"--- a/y.txt",
		"+++ b/y.txt",
		"@@ -1 +1,2 @@",
		" keep",
		"+new",
	)
	got, err := parseUnifiedDiff(in)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(got) != 2 || got[1].NewPath != "y.txt" || len(got[1].Hunks) != 1 {
		t.Fatalf("got %+v, want two files", got)
	}
	if got[1].Hunks[0].Lines[1] != (DiffLine{Type: DiffLineAdd, NewNum: 2, Text: "new"}) {
		t.Fatalf("y.txt line = %+v, want add new 2", got[1].Hunks[0].Lines[1])
	}
}

func TestParseUnifiedDiffHunkHeaderWithSection(t *testing.T) {
	in := diffJoin(
		"diff --git a/main.c b/main.c",
		"--- a/main.c",
		"+++ b/main.c",
		"@@ -2,3 +2,3 @@ int main() {",
		" a",
		"-b",
		"+c",
		" d",
	)
	got, err := parseUnifiedDiff(in)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if got[0].Hunks[0].Header != "@@ -2,3 +2,3 @@ int main() {" {
		t.Fatalf("header = %q, want the raw line", got[0].Hunks[0].Header)
	}
}

func TestParseUnifiedDiffTruncatedHunk(t *testing.T) {
	in := diffJoin(
		"diff --git a/x.txt b/x.txt",
		"--- a/x.txt",
		"+++ b/x.txt",
		"@@ -1,3 +1,3 @@",
		" a",
		"-b",
	)
	_, err := parseUnifiedDiff(in)
	if !errors.Is(err, ErrParseFailed) {
		t.Fatalf("err = %v, want parse_failed", err)
	}
}

func TestParseUnifiedDiffEmptyInput(t *testing.T) {
	got, err := parseUnifiedDiff([]byte(""))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("got %+v, want no files", got)
	}
}

func TestParseUnifiedDiffMalformed(t *testing.T) {
	cases := []struct {
		name        string
		input       []string
		skipPrelude bool // cases about "before any diff --git" must stay pre-section
	}{
		{"header outside section", []string{"@@ -1 +1 @@", " a"}, true},
		{"path header outside section", []string{"--- a/x.txt"}, true},
		{"garbage hunk header", []string{"@@ nonsense"}, false},
		{"hunk header missing ranges", []string{"@@ -1 +1"}, false},
		{"non-numeric start", []string{"@@ -x +1 @@", " a"}, false},
		{"negative start", []string{"@@ --1 +1 @@", " a"}, false},
		{"non-numeric count", []string{"@@ -1,z +1,z @@", " a"}, false},
		{"context beyond range", []string{"@@ -1,1 +1,1 @@", " a", " b"}, false},
		{"delete beyond range", []string{"@@ -0,0 +1,1 @@", "-x"}, false},
		{"add beyond range", []string{"@@ -1,1 +0,0 @@", "+x"}, false},
		{"empty line inside hunk", []string{"@@ -1,2 +1,2 @@", " a", ""}, false},
		{"unknown prefix inside hunk", []string{"@@ -1,2 +1,2 @@", "?x"}, false},
		{"marker without preceding line", []string{"@@ -0,0 +0,0 @@", `\ No newline at end of file`}, false},
		{"dangling escape", []string{`--- "a\"`}, false},
		{"unterminated quote", []string{`--- "a/x`}, false},
		{"unknown escape", []string{`--- "a/\9x"`}, false},
		{"non-marker backslash line", []string{"@@ -1,2 +1,2 @@", "+x", `\x`}, false},
		{"hunk header missing plus", []string{"@@ -1 2 @@"}, false},
		{"new range non-numeric", []string{"@@ -1,1 +x,1 @@", " a"}, false},
		{"garbage similarity", []string{"similarity index xx%"}, false},
		{"binary line missing pair", []string{"Binary files onlyone differ"}, false},
		{"binary line missing suffix", []string{"Binary files a/x and b/y"}, false},
		{"extended header outside section", []string{"similarity index 90%"}, true},
		{"rename from malformed quote", []string{`rename from "x\`}, false},
		{"rename to malformed quote", []string{"rename from x", `rename to "y\`}, false},
		{"binary endpoint malformed", []string{`Binary files "a/x\ and b/y differ`}, false},
		{"binary right endpoint malformed", []string{`Binary files a/x and "b/y\ differ`}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			in := diffJoin(tc.input...)
			if !tc.skipPrelude {
				in = diffJoin(append([]string{"diff --git a/x.txt b/x.txt"}, tc.input...)...)
			}
			_, err := parseUnifiedDiff(in)
			if !errors.Is(err, ErrParseFailed) {
				t.Fatalf("err = %v, want parse_failed", err)
			}
		})
	}
}

func TestParseUnifiedDiffTruncatedWithoutNewline(t *testing.T) {
	// killed git can stop mid-line: no final \n, so no empty trailing line
	in := "diff --git a/x.txt b/x.txt\n--- a/x.txt\n+++ b/x.txt\n@@ -1,3 +1,3 @@\n a\n-b"
	_, err := parseUnifiedDiff([]byte(in))
	if !errors.Is(err, ErrParseFailed) {
		t.Fatalf("err = %v, want parse_failed", err)
	}
}

func TestUnquoteGitPath(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"plain.txt", "plain.txt"},
		{`"wei\"rd.txt"`, `wei"rd.txt`},
		{`"back\\slash"`, `back\slash`},
		{`"a\ab"`, "a\ab"},
		{`"t\tb"`, "t\tb"},
		{`"n\nb"`, "n\nb"},
		{`"r\rb"`, "r\rb"},
		{`"f\fb"`, "f\fb"},
		{`"v\vb"`, "v\vb"},
		{`"b\bb"`, "b\bb"},
		{`"\303\274n\303\257code.txt"`, "\u00fcn\u00efcode.txt"}, // octal UTF-8: \303\274 = u-umlaut
		{`"o\7k"`, "o\x07k"}, // one-digit octal
		{`"t\12x"`, "t\nx"},  // two-digit octal
	}
	for _, tc := range cases {
		got, err := unquoteGitPath(tc.in)
		if err != nil {
			t.Fatalf("unquote %q: %v", tc.in, err)
		}
		if got != tc.want {
			t.Fatalf("unquote %q = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestParseUnifiedDiffAddedFile(t *testing.T) {
	in := diffJoin(
		"diff --git a/new.txt b/new.txt",
		"new file mode 100644",
		"index 0000000..b6fc4b6",
		"--- /dev/null",
		"+++ b/new.txt",
		"@@ -0,0 +1 @@",
		"+hello",
	)
	got, err := parseUnifiedDiff(in)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	f := got[0]
	if f.Change != ChangeAdded || f.OldPath != "" || f.NewPath != "new.txt" || f.NewMode != "100644" {
		t.Fatalf("got %+v, want added new.txt with mode 100644", f)
	}
}

func TestParseUnifiedDiffDeletedFile(t *testing.T) {
	in := diffJoin(
		"diff --git a/gone.txt b/gone.txt",
		"deleted file mode 100755",
		"index b6fc4b6..0000000",
		"--- a/gone.txt",
		"+++ /dev/null",
		"@@ -1 +0,0 @@",
		"-hello",
	)
	got, err := parseUnifiedDiff(in)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	f := got[0]
	if f.Change != ChangeDeleted || f.OldPath != "gone.txt" || f.NewPath != "" || f.OldMode != "100755" {
		t.Fatalf("got %+v, want deleted gone.txt with mode 100755", f)
	}
}

func TestParseUnifiedDiffEmptyAddedFileHasNoHunk(t *testing.T) {
	// real shape for adding a zero-byte file: headers only, no @@, no /dev/null
	in := diffJoin(
		"diff --git a/empty.txt b/empty.txt",
		"new file mode 100644",
		"index 0000000..e69de29",
	)
	got, err := parseUnifiedDiff(in)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	f := got[0]
	if f.Change != ChangeAdded || len(f.Hunks) != 0 {
		t.Fatalf("got %+v, want added with zero hunks", f)
	}
}

func TestParseUnifiedDiffRename(t *testing.T) {
	in := diffJoin(
		"diff --git a/c.txt b/renamed-c.txt",
		"similarity index 100%",
		"rename from c.txt",
		"rename to renamed-c.txt",
	)
	got, err := parseUnifiedDiff(in)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	f := got[0]
	if f.Change != ChangeRenamed || f.Similarity != 100 || f.OldPath != "c.txt" || f.NewPath != "renamed-c.txt" {
		t.Fatalf("got %+v, want R100 c.txt -> renamed-c.txt", f)
	}
}

func TestParseUnifiedDiffRenameWithEdit(t *testing.T) {
	in := diffJoin(
		"diff --git a/old.txt b/new.txt",
		"similarity index 87%",
		"rename from old.txt",
		"rename to new.txt",
		"--- a/old.txt",
		"+++ b/new.txt",
		"@@ -1,2 +1,2 @@",
		" same",
		"-content",
		"+CONTENT",
	)
	got, err := parseUnifiedDiff(in)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	f := got[0]
	if f.Change != ChangeRenamed || f.Similarity != 87 || f.OldPath != "old.txt" || f.NewPath != "new.txt" {
		t.Fatalf("got %+v, want R87 old.txt -> new.txt", f)
	}
	if len(f.Hunks) != 1 || f.Hunks[0].Lines[2].Text != "CONTENT" {
		t.Fatalf("hunks = %+v, want one edited hunk", f.Hunks)
	}
}

func TestParseUnifiedDiffModeOnly(t *testing.T) {
	in := diffJoin(
		"diff --git a/new.txt b/new.txt",
		"old mode 100644",
		"new mode 100755",
	)
	got, err := parseUnifiedDiff(in)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	f := got[0]
	if f.Change != ChangeModified || f.OldMode != "100644" || f.NewMode != "100755" || len(f.Hunks) != 0 {
		t.Fatalf("got %+v, want mode-only modified with both modes and no hunks", f)
	}
}

func TestParseUnifiedDiffBinary(t *testing.T) {
	in := diffJoin(
		"diff --git a/bin.dat b/bin.dat",
		"index 4d15381..0f49c4a 100644",
		"Binary files a/bin.dat and b/bin.dat differ",
	)
	got, err := parseUnifiedDiff(in)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	f := got[0]
	if !f.Binary || f.OldPath != "bin.dat" || f.NewPath != "bin.dat" || len(f.Hunks) != 0 {
		t.Fatalf("got %+v, want binary bin.dat with no hunks", f)
	}
}
