package git

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
)

// subFixture: bare seed + work clone pushed; super with README and one
// registered submodule sub1. Local paths need the file protocol opt-in,
// which fixtures grant explicitly (production stays flag-neutral).
func subFixture(t *testing.T) (super, seed string) {
	t.Helper()
	tmp := t.TempDir()
	seed = filepath.Join(tmp, "seed.git")
	gitOut(t, tmp, "init", "--bare", "-b", "main", seed)

	work := filepath.Join(tmp, "work")
	gitOut(t, tmp, "clone", seed, work)
	setGitIdentity(t, work)
	writeFile(t, work, "lib.txt", "lib\n")
	commitAll(t, work, "lib")
	gitOut(t, work, "push", "-q", "origin", "main")

	super = filepath.Join(tmp, "super")
	gitOut(t, tmp, "init", "-b", "main", super)
	setGitIdentity(t, super)
	writeFile(t, super, "README", "top\n")
	commitAll(t, super, "top")
	runGitMust(t, super, "-c", "protocol.file.allow=always", "submodule", "add", "-q", "../work", "sub1")
	gitOut(t, super, "commit", "-qm", "add sub1")
	return super, seed
}

func runGitMust(t *testing.T, dir string, args ...string) {
	t.Helper()
	if _, _, err := runGit(context.Background(), dir, args...); err != nil {
		t.Fatalf("git %v: %v", args, err)
	}
}

func subState(t *testing.T, super, path string) SubmoduleState {
	t.Helper()
	list, err := openRepo(t, super).Submodules(context.Background())
	if err != nil {
		t.Fatalf("Submodules: %v", err)
	}
	for _, s := range list {
		if s.Path == path {
			return s.State
		}
	}
	t.Fatalf("submodule %q missing from %+v", path, list)
	return ""
}

func TestSubmodulesClean(t *testing.T) {
	super, _ := subFixture(t)
	list, err := openRepo(t, super).Submodules(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 {
		t.Fatalf("got %+v, want one", list)
	}
	s := list[0]
	if s.Path != "sub1" || s.State != SubmoduleOK {
		t.Fatalf("got %+v, want sub1 ok", s)
	}
	if len(s.SHA) != 40 {
		t.Fatalf("SHA = %q, want a full oid", s.SHA)
	}
	want := gitOut(t, filepath.Join(super, "sub1"), "rev-parse", "HEAD")
	if s.SHA != want {
		t.Fatalf("SHA = %q, want checked out %q", s.SHA, want)
	}
	if s.Describe != "heads/main" {
		t.Fatalf("Describe = %q, want heads/main", s.Describe)
	}
}

func TestSubmodulesPointerMoved(t *testing.T) {
	super, _ := subFixture(t)
	sub := filepath.Join(super, "sub1")
	writeFile(t, sub, "extra.txt", "x\n")
	setGitIdentity(t, sub)
	commitAll(t, sub, "extra")
	if got := subState(t, super, "sub1"); got != SubmoduleModified {
		t.Fatalf("state = %q, want modified after sub-side commit", got)
	}
}

func TestSubmodulesUninitialized(t *testing.T) {
	super, _ := subFixture(t)
	gitOut(t, super, "submodule", "deinit", "-q", "sub1")
	if got := subState(t, super, "sub1"); got != SubmoduleUninitialized {
		t.Fatalf("state = %q, want uninitialized", got)
	}
}

func TestSubmodulesConflict(t *testing.T) {
	ctx := context.Background()
	super, _ := subFixture(t)
	sub := filepath.Join(super, "sub1")
	setGitIdentity(t, sub)

	// two sibling sub commits (divergent, not ancestor/descendant:
	// merge auto-picks descendants, which would silently dodge conflict)
	fork := gitOut(t, sub, "rev-parse", "HEAD")
	writeFile(t, sub, "b.txt", "b\n")
	commitAll(t, sub, "sub b")

	// main records b; use-a records a; merging them conflicts the gitlink
	gitOut(t, super, "add", "sub1")
	gitOut(t, super, "commit", "-qm", "points b")
	gitOut(t, super, "checkout", "-qb", "use-a", "HEAD~1")
	runGitMust(t, sub, "checkout", "-q", fork)
	writeFile(t, sub, "a.txt", "a\n")
	commitAll(t, sub, "sub a")
	gitOut(t, super, "add", "sub1")
	gitOut(t, super, "commit", "-qm", "points a")
	gitOut(t, super, "checkout", "-q", "main")

	if _, _, err := runGit(ctx, super, "merge", "-q", "use-a"); err == nil {
		t.Fatal("fixture merge must conflict on the gitlink")
	}
	if got := subState(t, super, "sub1"); got != SubmoduleConflict {
		t.Fatalf("state = %q, want conflict", got)
	}
}

func TestParseSubmoduleStatusMalformed(t *testing.T) {
	long := " " + string(make([]byte, 0))
	oid := "0123456789012345678901234567890123456789"
	cases := []string{
		"short line",
		long,
		"X" + oid + " sub1 ()",
		" " + strings.Repeat("z", 40) + " sub1 ()",
		" " + oid,
		" " + oid + " ",
		" " + oid + " x)",
	}
	for _, in := range cases {
		if _, err := parseSubmoduleStatus(in); err == nil {
			t.Fatalf("parseSubmoduleStatus(%q) = nil, want parse failure", in)
		}
	}
	ok := " " + oid + " sub1 (heads/main)\n-" + oid + " plain\nU" + strings.Repeat("0", 40) + " conflicted\n"
	list, err := parseSubmoduleStatus(ok)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 3 || list[0].Describe != "heads/main" || list[1].Path != "plain" || list[1].Describe != "" {
		t.Fatalf("good parse = %+v", list)
	}
	if list[2].State != SubmoduleConflict || list[2].SHA != strings.Repeat("0", 40) || list[2].Describe != "" {
		t.Fatalf("conflict line parsed as %+v", list[2])
	}
}

func TestSubmodulesCtxKill(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := openRepo(t, subFixtureOnly(t)).Submodules(ctx); !errors.Is(err, ErrTimeout) {
		t.Fatalf("err = %v, want timeout", err)
	}
}

func subFixtureOnly(t *testing.T) string {
	t.Helper()
	super, _ := subFixture(t)
	return super
}
