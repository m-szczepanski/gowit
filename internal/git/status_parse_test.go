package git

import (
	"strings"
	"testing"
)

func joinZ(segs ...string) []byte {
	return []byte(strings.Join(append(segs, ""), "\x00"))
}

// Fixtures use the record shapes from git's porcelain v2 spec, verified
// against real `git status --porcelain=v2 --branch -z` output.

func TestParseStatusV2Headers(t *testing.T) {
	out := joinZ(
		"# branch.oid 83e92cf7e3be63224c6ce486c2bf0d32e0adea13",
		"# branch.head main",
		"# branch.upstream origin/main",
		"# branch.ab +2 -1",
		"# branch.future-field whatever",
	)
	res, err := parseStatusV2(out)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	b := res.Branch
	if b.Oid != "83e92cf7e3be63224c6ce486c2bf0d32e0adea13" || b.Head != "main" || b.Detached {
		t.Fatalf("branch = %+v", b)
	}
	if b.Upstream != "origin/main" || b.Ahead != 2 || b.Behind != 1 {
		t.Fatalf("tracking = %+v, want origin/main +2 -1", b)
	}
	if len(res.Files) != 0 {
		t.Fatalf("files = %v, want none", res.Files)
	}
}

func TestParseStatusV2DetachedHeader(t *testing.T) {
	out := joinZ(
		"# branch.oid 83e92cf7e3be63224c6ce486c2bf0d32e0adea13",
		"# branch.head (detached)",
	)
	res, err := parseStatusV2(out)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if !res.Branch.Detached || res.Branch.Head != "" {
		t.Fatalf("branch = %+v, want detached without a name", res.Branch)
	}
}

func TestParseStatusV2IgnoredRecord(t *testing.T) {
	out := joinZ("# branch.oid abc", "# branch.head main", "! some/ignored.txt")
	res, err := parseStatusV2(out)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	f := res.Files[0]
	if f.Path != "some/ignored.txt" || !f.Ignored || f.Change() != ChangeIgnored {
		t.Fatalf("ignored = %+v", f)
	}
	if f.Staged() || f.Unstaged() {
		t.Fatalf("ignored entry classified as change: %+v", f)
	}
}

func TestParseStatusV2SubmoduleAndCopyRecords(t *testing.T) {
	out := joinZ(
		"# branch.oid abc",
		"# branch.head main",
		"1 M. M... 160000 160000 160000 a1b2c3 d4e5f6 submodule-dir",
		"2 C. N... 100644 100644 100644 a1b2c3 d4e5f6 C88 copy.txt",
		"source.txt",
	)
	res, err := parseStatusV2(out)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	sub := res.Files[0]
	if sub.Path != "submodule-dir" || sub.Submodule != "M..." || sub.XY != "M." || sub.Change() != ChangeModified {
		t.Fatalf("submodule entry = %+v", sub)
	}
	cp := res.Files[1]
	if cp.Path != "copy.txt" || cp.OrigPath != "source.txt" || cp.XY != "C." || cp.Change() != ChangeCopied {
		t.Fatalf("copy entry = %+v", cp)
	}
}

func TestParseStatusV2RejectsMalformedRecords(t *testing.T) {
	cases := map[string][]byte{
		"unknown record type":     joinZ("# branch.head main", "3 weird record"),
		"truncated ordinary":      joinZ("1 M. N... 100644 100644"),
		"truncated rename":        joinZ("2 R. N... 100644 100644 100644 a b R100"),
		"rename without original": joinZ("2 R. N... 100644 100644 100644 a b R100 to.txt"),
		"bad branch.ab":           joinZ("# branch.ab notanumber -1"),
		"empty branch.ab minus":   joinZ("# branch.ab +1"),
		"bad ab count":            joinZ("# branch.ab +x -1"),
	}
	for name, out := range cases {
		_, err := parseStatusV2(out)
		if err == nil {
			t.Fatalf("%s: want error", name)
		}
		if !strings.Contains(err.Error(), "malformed") {
			t.Fatalf("%s: error = %v, want malformed", name, err)
		}
	}
}

func TestParseStatusV2RejectsMalformedHeadersAndUnmerged(t *testing.T) {
	cases := map[string][]byte{
		"header without value": joinZ("# branch.oid"),
		"bad ab minus count":   joinZ("# branch.ab +1 bad"),
		"truncated unmerged":   joinZ("u UU N... 100644 100644 100644"),
	}
	for name, out := range cases {
		if _, err := parseStatusV2(out); err == nil || !strings.Contains(err.Error(), "malformed") {
			t.Fatalf("%s: err = %v, want malformed", name, err)
		}
	}
}
