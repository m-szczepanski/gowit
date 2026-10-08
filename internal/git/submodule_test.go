package git

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// subFixture: bare seed + work clone pushed; super with README and one
// registered submodule sub1. Local paths need the file protocol opt-in,
// which fixtures grant explicitly (production stays flag-neutral).
// isolateGitConfig pins a fixture-owned global config: CI runners differ
// (Windows defaults core.autocrlf=true, which rewrites checked-out bytes;
// some runners carry no identity at all), and repos created inside git
// calls, like recursively cloned submodules, cannot be configured first.
func isolateGitConfig(t *testing.T) {
	t.Helper()
	file := filepath.Join(t.TempDir(), "config")
	body := "[user]\n\tname = test\n\temail = t@t\n[core]\n\tautocrlf = false\n"
	if err := os.WriteFile(file, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GIT_CONFIG_GLOBAL", file)
	t.Setenv("GIT_CONFIG_SYSTEM", filepath.Join(t.TempDir(), "missing"))
}

func subFixture(t *testing.T) (super, seed string) {
	t.Helper()
	isolateGitConfig(t)
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
	// the submodule is a fresh clone: no author identity until set
	setGitIdentity(t, filepath.Join(super, "sub1"))
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

func TestSubmoduleUpdateInit(t *testing.T) {
	ctx := context.Background()
	super, _ := subFixture(t)
	gitOut(t, super, "submodule", "deinit", "-q", "sub1")
	if got := subState(t, super, "sub1"); got != SubmoduleUninitialized {
		t.Fatalf("precondition: %q", got)
	}
	if err := openRepo(t, super).SubmoduleUpdateInit(ctx, false); err != nil {
		t.Fatalf("SubmoduleUpdateInit: %v", err)
	}
	if got := subState(t, super, "sub1"); got != SubmoduleOK {
		t.Fatalf("after init: %q, want ok", got)
	}
	if got := fileContent(t, filepath.Join(super, "sub1"), "lib.txt"); got != "lib\n" {
		t.Fatalf("work tree not restored, lib.txt = %q", got)
	}
}

func TestSubmoduleUpdateInitRecursive(t *testing.T) {
	ctx := context.Background()
	super, _ := subFixtureWithNested(t)
	gitOut(t, super, "submodule", "deinit", "-q", "--all")
	if err := openRepo(t, super).SubmoduleUpdateInit(ctx, true); err != nil {
		t.Fatalf("recursive init: %v", err)
	}
	if state := nestedState(t, super); state != SubmoduleOK {
		t.Fatalf("nested after recursive init = %q, want ok", state)
	}
	if got := fileContent(t, filepath.Join(super, "sub1", "inner"), "deep.txt"); got != "deep\n" {
		t.Fatalf("nested file missing: %q", got)
	}
}

func nestedState(t *testing.T, super string) SubmoduleState {
	t.Helper()
	out, _, err := runGit(context.Background(), super, "submodule", "status", "--recursive")
	if err != nil {
		t.Fatal(err)
	}
	list, err := parseSubmoduleStatus(string(out))
	if err != nil {
		t.Fatalf("recursive status unparsable: %v\n%s", err, out)
	}
	for _, s := range list {
		if s.Path == "sub1/inner" {
			return s.State
		}
	}
	t.Fatalf("nested entry missing in:\n%s", out)
	return ""
}

// subFixtureWithNested extends subFixture: work carries its own submodule
// inner (from seed2), so super has two levels. Repo-local
// protocol.file.allow lets recursive clones of sibling paths proceed,
// mirroring what a user opts into for local development trees.
func subFixtureWithNested(t *testing.T) (super, seed string) {
	t.Helper()
	allowFileTransport(t)
	super, seed = subFixture(t)
	tmp := filepath.Dir(super)
	seed2 := filepath.Join(tmp, "seed2.git")
	gitOut(t, tmp, "init", "--bare", "-b", "main", seed2)
	deep := filepath.Join(tmp, "deep")
	gitOut(t, tmp, "clone", seed2, deep)
	setGitIdentity(t, deep)
	writeFile(t, deep, "deep.txt", "deep\n")
	commitAll(t, deep, "deep")
	gitOut(t, deep, "push", "-q", "origin", "main")

	sub := filepath.Join(super, "sub1")
	// absolute path: a relative <repository> in submodule add resolves
	// against the superproject's remote, not the working directory
	runGitMust(t, sub, "-c", "protocol.file.allow=always", "submodule", "add", "-q", deep, "inner")
	gitOut(t, sub, "commit", "-qm", "add inner")
	gitOut(t, super, "add", "sub1")
	gitOut(t, super, "commit", "-qm", "sub records inner")
	return super, seed
}

// allowFileTransport opts the test process (and every git child it
// spawns) into the local file transport via the documented env config;
// production code never widens this policy itself.
func allowFileTransport(t *testing.T) {
	t.Helper()
	t.Setenv("GIT_CONFIG_COUNT", "1")
	t.Setenv("GIT_CONFIG_KEY_0", "protocol.file.allow")
	t.Setenv("GIT_CONFIG_VALUE_0", "always")
}

func TestSubmoduleUpdatePath(t *testing.T) {
	ctx := context.Background()
	super, _ := subFixture(t)
	gitOut(t, super, "submodule", "deinit", "-q", "sub1")
	r := openRepo(t, super)
	if err := r.SubmoduleUpdatePath(ctx, "sub1"); err != nil {
		t.Fatalf("update by path: %v", err)
	}
	if got := subState(t, super, "sub1"); got != SubmoduleOK {
		t.Fatalf("state = %q", got)
	}
	if err := r.SubmoduleUpdatePath(ctx, "../outside"); !errors.Is(err, ErrValidationFailed) {
		t.Fatalf("escape: %v", err)
	}
	if err := r.SubmoduleUpdatePath(ctx, "nope"); !errors.Is(err, ErrValidationFailed) {
		t.Fatalf("unknown path: %v, want validation", err)
	}
}

func TestOpenSubmodule(t *testing.T) {
	ctx := context.Background()
	super, _ := subFixture(t)
	r := openRepo(t, super)

	sub, err := r.OpenSubmodule(ctx, "sub1")
	if err != nil {
		t.Fatalf("OpenSubmodule: %v", err)
	}
	if !strings.HasSuffix(sub.Path(), "sub1") {
		t.Fatalf("path = %q", sub.Path())
	}
	st, err := sub.Status(ctx)
	if err != nil || len(st.Files) != 0 {
		t.Fatalf("status inside submodule: %+v %v", st, err)
	}

	if _, err := r.OpenSubmodule(ctx, "README"); !errors.Is(err, ErrValidationFailed) {
		t.Fatalf("unregistered path: %v", err)
	}
	gitOut(t, super, "submodule", "deinit", "-q", "sub1")
	if _, err := r.OpenSubmodule(ctx, "sub1"); !errors.Is(err, ErrNotARepository) {
		t.Fatalf("uninitialized submodule: %v, want not_a_repository", err)
	} else if !strings.Contains(err.Error(), "not initialized") {
		t.Fatalf("message should explain the uninitialized state: %v", err)
	}
}

func TestSubmoduleUpdatePathCtxKill(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	super, _ := subFixture(t)
	if err := openRepo(t, super).SubmoduleUpdatePath(ctx, "sub1"); !errors.Is(err, ErrTimeout) {
		t.Fatalf("err = %v, want timeout", err)
	}
}

func TestSubmoduleAddLifecycle(t *testing.T) {
	ctx := context.Background()
	allowFileTransport(t)
	super, _ := subFixture(t)
	tmp := filepath.Dir(super)
	deep2 := filepath.Join(tmp, "deep2")
	gitOut(t, tmp, "init", "-b", "main", deep2)
	setGitIdentity(t, deep2)
	writeFile(t, deep2, "d2.txt", "two\n")
	commitAll(t, deep2, "two")

	r := openRepo(t, super)
	if err := r.SubmoduleAdd(ctx, deep2, "sub2"); err != nil {
		t.Fatalf("SubmoduleAdd: %v", err)
	}
	if got := subState(t, super, "sub2"); got != SubmoduleOK {
		t.Fatalf("state after add = %q", got)
	}
	// add stages but does not commit: the user owns the history
	out := gitOut(t, super, "diff", "--cached", "--name-only")
	if !strings.Contains(out, ".gitmodules") || !strings.Contains(out, "sub2") {
		t.Fatalf("staged records = %q, want .gitmodules and sub2", out)
	}
	gitOut(t, super, "commit", "-qm", "add sub2")

	if err := r.SubmoduleDeinit(ctx, "sub2"); err != nil {
		t.Fatalf("deinit: %v", err)
	}
	if got := subState(t, super, "sub2"); got != SubmoduleUninitialized {
		t.Fatalf("state after deinit = %q", got)
	}
	if err := r.SubmoduleRemove(ctx, "sub2"); err != nil {
		t.Fatalf("remove: %v", err)
	}
	list, err := r.Submodules(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range list {
		if s.Path == "sub2" {
			t.Fatalf("sub2 survived removal: %+v", list)
		}
	}
	leftovers, _, cfgErr := runGit(context.Background(), super, "config", "-f", ".gitmodules", "--get-regexp", "submodule\\.sub2\\.")
	if strings.TrimSpace(string(leftovers)) != "" {
		t.Fatalf(".gitmodules leftovers: %q", leftovers)
	}
	if cfgErr != nil && !errors.Is(cfgErr, ErrCommandFailed) {
		t.Fatalf("config probe: %v", cfgErr)
	}
	if _, err := os.Stat(filepath.Join(super, ".git", "modules", "sub2")); !os.IsNotExist(err) {
		t.Fatalf("module git dir left behind: %v", err)
	}
	if got := fileContent(t, super, "README"); got != "top\n" {
		t.Fatalf("removal touched the work tree: README = %q", got)
	}
}

func TestSubmoduleLifecycleGuards(t *testing.T) {
	ctx := context.Background()
	super, _ := subFixture(t)
	r := openRepo(t, super)
	if err := r.SubmoduleAdd(ctx, "https://example.com/x.git", "../escape"); !errors.Is(err, ErrValidationFailed) {
		t.Fatalf("add escape path: %v", err)
	}
	if err := r.SubmoduleAdd(ctx, "-uhttps://evil", "ok/dir"); !errors.Is(err, ErrValidationFailed) {
		t.Fatalf("add dash url: %v", err)
	}
	if err := r.SubmoduleDeinit(ctx, "nope"); !errors.Is(err, ErrValidationFailed) {
		t.Fatalf("deinit unregistered: %v", err)
	}
	if err := r.SubmoduleRemove(ctx, "nope"); !errors.Is(err, ErrValidationFailed) {
		t.Fatalf("remove unregistered: %v", err)
	}
}

func TestSubmoduleRemoveDirtyPointerFailsAtDeinit(t *testing.T) {
	ctx := context.Background()
	super, _ := subFixture(t)
	sub := filepath.Join(super, "sub1")
	setGitIdentity(t, sub)
	writeFile(t, sub, "new.txt", "n\n")
	commitAll(t, sub, "sub moved")

	r := openRepo(t, super)
	if err := r.SubmoduleRemove(ctx, "sub1"); !errors.Is(err, ErrCommandFailed) {
		t.Fatalf("err = %v, want deinit refusal on the dirty pointer", err)
	}
	// refusal must leave everything intact for the user to salvage
	if got := subState(t, super, "sub1"); got != SubmoduleModified {
		t.Fatalf("state after refusal = %q, want still modified", got)
	}
}

func TestSubmoduleRemoveUnreadableModuleDirPropagates(t *testing.T) {
	skipWithoutUnixPerms(t)
	ctx := context.Background()
	super, _ := subFixture(t)
	// deinit first while permissions allow: Remove's own re-deinit is
	// then a no-op and git rm exits clean, so the only way the call can
	// fail is the module-data removal this test is about
	gitOut(t, super, "submodule", "deinit", "-q", "sub1")
	module := filepath.Join(super, ".git", "modules", "sub1")
	if err := os.Chmod(module, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(module, 0o755) })
	err := openRepo(t, super).SubmoduleRemove(ctx, "sub1")
	if err == nil {
		t.Fatal("want module data removal failure propagated")
	}
	if _, statErr := os.Stat(module); statErr != nil {
		t.Fatalf("failed remove should not have forced the deletion away: %v", statErr)
	}
}

func TestSubmoduleIgnoresUnrelatedGitmodulesSections(t *testing.T) {
	ctx := context.Background()
	super, _ := subFixture(t)
	gitOut(t, super, "config", "-f", ".gitmodules", "submodule.junk.path", "elsewhere")
	gitOut(t, super, "config", "-f", ".gitmodules", "submodule.junk.url", "https://example.invalid/x")
	// git rm insists on a clean .gitmodules, so the unrelated section is
	// committed exactly like a user's would be
	gitOut(t, super, "add", ".gitmodules")
	gitOut(t, super, "commit", "-qm", "unrelated section")

	r := openRepo(t, super)
	if err := r.SubmoduleRemove(ctx, "sub1"); err != nil {
		t.Fatalf("remove with an unrelated section present: %v", err)
	}
	list, err := r.Submodules(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range list {
		if s.Path == "sub1" {
			t.Fatalf("sub1 survived: %+v", list)
		}
	}
	if got := gitOut(t, super, "config", "-f", ".gitmodules", "submodule.junk.path"); got != "elsewhere" {
		t.Fatalf("unrelated section must survive removal of another: %q", got)
	}
}

func TestSubmoduleRemoveRefusedByUnstagedGitmodules(t *testing.T) {
	ctx := context.Background()
	super, _ := subFixture(t)
	r := openRepo(t, super)
	// deinit first so its own .gitmodules check passes; an unstaged edit
	// introduced afterwards lets the repeated deinit through but makes
	// git rm refuse, so Remove surfaces the refusal at the second step
	if err := r.SubmoduleDeinit(ctx, "sub1"); err != nil {
		t.Fatal(err)
	}
	gitOut(t, super, "config", "-f", ".gitmodules", "submodule.junk.path", "elsewhere")
	err := r.SubmoduleRemove(ctx, "sub1")
	if !errors.Is(err, ErrCommandFailed) {
		t.Fatalf("err = %v, want git rm refusal surfaced", err)
	}
}
