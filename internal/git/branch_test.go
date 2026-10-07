package git

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

// seededBranches builds a bare origin plus a clone with a deliberate mix:
// main synced with origin, feature/a ahead 1 and behind 1, noUp local with
// no upstream, solo remote-only, gone tracked but deleted on origin and
// pruned from refs/remotes. Returns the clone dir.
func seededBranches(t *testing.T) (dir string) {
	t.Helper()
	tmp := t.TempDir()
	origin := filepath.Join(tmp, "origin.git")
	gitOut(t, tmp, "init", "--bare", "-b", "main", origin)

	gitOut(t, tmp, "clone", origin, "clone")
	dir = filepath.Join(tmp, "clone")
	setGitIdentity(t, dir)
	writeFile(t, dir, "f.txt", "one\n")
	commitAll(t, dir, "c1")
	gitOut(t, dir, "push", "-u", "origin", "main")
	gitOut(t, dir, "branch", "feature/a")
	gitOut(t, dir, "push", "-u", "origin", "feature/a")
	gitOut(t, dir, "branch", "gone")
	gitOut(t, dir, "push", "-u", "origin", "gone")
	gitOut(t, dir, "checkout", "-b", "noUp")
	writeFile(t, dir, "g.txt", "no upstream\n")
	commitAll(t, dir, "c-noUp")
	gitOut(t, dir, "checkout", "main")

	gitOut(t, tmp, "clone", origin, "clone2")
	second := filepath.Join(tmp, "clone2")
	setGitIdentity(t, second)
	gitOut(t, second, "checkout", "feature/a")
	writeFile(t, second, "f.txt", "two\n")
	commitAll(t, second, "c2-remote")
	gitOut(t, second, "push")
	gitOut(t, second, "checkout", "-b", "solo")
	writeFile(t, second, "s.txt", "solo\n")
	commitAll(t, second, "c-solo")
	gitOut(t, second, "push", "-u", "origin", "solo")
	gitOut(t, second, "push", "origin", "--delete", "gone")

	gitOut(t, dir, "checkout", "feature/a")
	writeFile(t, dir, "f.txt", "three\n")
	commitAll(t, dir, "c3-local")
	gitOut(t, dir, "checkout", "main")
	gitOut(t, dir, "fetch", "origin", "--prune")
	return dir
}

func branchesByName(t *testing.T, r *Repo, scope BranchScope) map[string]Branch {
	t.Helper()
	list, err := r.Branches(context.Background(), scope)
	if err != nil {
		t.Fatalf("Branches: %v", err)
	}
	byName := make(map[string]Branch, len(list))
	for _, b := range list {
		byName[b.Name] = b
	}
	return byName
}

func TestBranchesLocalScope(t *testing.T) {
	dir := seededBranches(t)
	got := branchesByName(t, openRepo(t, dir), ScopeLocal)

	wantNames := []string{"feature/a", "gone", "main", "noUp"}
	if len(got) != len(wantNames) {
		t.Fatalf("locals = %v, want %v", got, wantNames)
	}
	for _, n := range wantNames {
		if _, ok := got[n]; !ok {
			t.Fatalf("missing local %q", n)
		}
	}
	if _, ok := got["solo"]; ok {
		t.Fatalf("remote-only branch leaked into locals")
	}

	a := got["feature/a"]
	if !a.IsLocal || a.IsRemote {
		t.Fatalf("feature/a flags: local=%v remote=%v", a.IsLocal, a.IsRemote)
	}
	if a.Ref != "refs/heads/feature/a" {
		t.Fatalf("Ref = %q", a.Ref)
	}
	if a.Upstream != "origin/feature/a" {
		t.Fatalf("Upstream = %q, want origin/feature/a", a.Upstream)
	}
	if a.Ahead != 1 || a.Behind != 1 {
		t.Fatalf("feature/a ahead/behind = %d/%d, want 1/1", a.Ahead, a.Behind)
	}
	if a.Subject != "c3-local" {
		t.Fatalf("Subject = %q, want c3-local", a.Subject)
	}

	main := got["main"]
	if main.Ahead != 0 || main.Behind != 0 {
		t.Fatalf("main ahead/behind = %d/%d, want 0/0", main.Ahead, main.Behind)
	}
	if main.Subject != "c1" {
		t.Fatalf("main Subject = %q, want c1", main.Subject)
	}

	noUp := got["noUp"]
	if noUp.Upstream != "" || noUp.Ahead != 0 || noUp.Behind != 0 {
		t.Fatalf("noUp = %+v, want no upstream and 0/0", noUp)
	}

	gone := got["gone"]
	if gone.Upstream != "origin/gone" {
		t.Fatalf("gone Upstream = %q, want origin/gone", gone.Upstream)
	}
	if gone.Ahead != 0 || gone.Behind != 0 {
		t.Fatalf("gone ahead/behind = %d/%d, want 0/0 when upstream is gone", gone.Ahead, gone.Behind)
	}

	head := gitOut(t, dir, "rev-parse", "refs/heads/feature/a")
	if a.Head != head {
		t.Fatalf("Head = %q, want %q", a.Head, head)
	}
}

func TestBranchesRemoteScope(t *testing.T) {
	dir := seededBranches(t)
	got := branchesByName(t, openRepo(t, dir), ScopeRemote)

	for _, n := range []string{"origin/main", "origin/feature/a", "origin/solo"} {
		if _, ok := got[n]; !ok {
			t.Fatalf("missing remote %q in %v", n, got)
		}
	}
	if _, ok := got["origin/gone"]; ok {
		t.Fatalf("pruned remote ref came back")
	}
	r := got["origin/solo"]
	if r.IsLocal || !r.IsRemote {
		t.Fatalf("origin/solo flags: local=%v remote=%v", r.IsLocal, r.IsRemote)
	}
	if r.Ref != "refs/remotes/origin/solo" {
		t.Fatalf("Ref = %q", r.Ref)
	}
	if r.Upstream != "" {
		t.Fatalf("remote branch Upstream = %q, want empty", r.Upstream)
	}
	if r.Subject != "c-solo" {
		t.Fatalf("Subject = %q, want c-solo", r.Subject)
	}
}

func TestBranchesAllIsSortedUnion(t *testing.T) {
	ctx := context.Background()
	dir := seededBranches(t)
	r := openRepo(t, dir)
	all, err := r.Branches(ctx, ScopeAll)
	if err != nil {
		t.Fatal(err)
	}
	local, _ := r.Branches(ctx, ScopeLocal)
	remote, _ := r.Branches(ctx, ScopeRemote)
	if len(all) != len(local)+len(remote) {
		t.Fatalf("all=%d, want %d total", len(all), len(local)+len(remote))
	}
	var refs []string
	for _, b := range all {
		refs = append(refs, b.Ref)
	}
	for i := 1; i < len(refs); i++ {
		if refs[i-1] > refs[i] {
			t.Fatalf("refs not sorted: %v", refs)
		}
	}
}

func TestBranchesCommitterDate(t *testing.T) {
	dir := seededBranches(t)
	out := gitOut(t, dir, "show", "-s", "--format=%ct", "refs/heads/noUp")
	sec, err := strconv.ParseInt(out, 10, 64)
	if err != nil {
		t.Fatal(err)
	}
	got := branchesByName(t, openRepo(t, dir), ScopeLocal)["noUp"]
	if want := time.Unix(sec, 0).UTC(); !got.CommitterDate.Equal(want) {
		t.Fatalf("CommitterDate = %v, want %v", got.CommitterDate, want)
	}
}

func TestParseBranchRecordsMalformed(t *testing.T) {
	const oid = "0123456789012345678901234567890123456789"
	cases := []string{
		"refs/heads/x\x00abc",
		"\x00\x00\x00\x00\x00",
		"refs/heads/a\x00\x00\x00\x00s\x001700000000",
		"refs/heads/a\x00" + oid + "\x00origin/a\x00[ahead]\x00s\x001700000000",
		"refs/heads/a\x00" + oid + "\x00origin/a\x00[ahead x]\x00s\x001700000000",
		"refs/heads/a\x00" + oid + "\x00origin/a\x00[diverged 1]\x00s\x001700000000",
		"refs/heads/a\x00" + oid + "\x00\x00\x00s\x00not-a-time",
		"refs/tags/v1\x00" + oid + "\x00\x00\x00tag\x001700000000",
	}
	for _, in := range cases {
		if _, err := parseBranchRecords(in); err == nil {
			t.Fatalf("parseBranchRecords(%q) = nil error, want parse failure", in)
		} else if ge, ok := err.(*GitError); !ok || ge.Code != CodeParseFailed {
			t.Fatalf("parseBranchRecords(%q) err = %v, want parse_failed", in, err)
		}
	}
}

func TestParseBranchRecordsSubjectWithSpecials(t *testing.T) {
	line := "refs/heads/y\x00" + strings.Repeat("0", 40) + "\x00origin/y\x00[ahead 2]\x00Merge x: a-b, #1 & \"q\"\x001700000000\n"
	list, err := parseBranchRecords(line)
	if err != nil {
		t.Fatal(err)
	}
	b := list[0]
	if b.Subject != `Merge x: a-b, #1 & "q"` {
		t.Fatalf("Subject = %q", b.Subject)
	}
	if b.Ahead != 2 || b.Behind != 0 {
		t.Fatalf("ahead/behind = %d/%d, want 2/0", b.Ahead, b.Behind)
	}
	if !b.CommitterDate.Equal(time.Unix(1700000000, 0).UTC()) {
		t.Fatalf("CommitterDate = %v", b.CommitterDate)
	}
}

func TestBranchesEmptyRepo(t *testing.T) {
	list, err := openRepo(t, initRepo(t)).Branches(context.Background(), ScopeAll)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 0 {
		t.Fatalf("got %+v, want none", list)
	}
}

func TestBranchesCtxKill(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := openRepo(t, seededBranches(t)).Branches(ctx, ScopeAll); !errors.Is(err, ErrTimeout) {
		t.Fatalf("err = %v, want timeout", err)
	}
}

func TestBranchesMarksCurrent(t *testing.T) {
	dir := seededBranches(t)
	r := openRepo(t, dir)
	all, err := r.Branches(context.Background(), ScopeAll)
	if err != nil {
		t.Fatal(err)
	}
	current := 0
	for _, b := range all {
		if b.IsCurrent {
			current++
			if b.Name != "main" || !b.IsLocal {
				t.Fatalf("IsCurrent on %+v, want local main only", b)
			}
		}
	}
	if current != 1 {
		t.Fatalf("IsCurrent count = %d, want exactly 1 in %+v", current, all)
	}
}

func TestBranchesDetachedHasNoCurrent(t *testing.T) {
	dir := seededBranches(t)
	gitOut(t, dir, "checkout", "-q", "--detach")
	r := openRepo(t, dir)
	all, err := r.Branches(context.Background(), ScopeAll)
	if err != nil {
		t.Fatal(err)
	}
	for _, b := range all {
		if b.IsCurrent {
			t.Fatalf("IsCurrent set while detached: %+v", b)
		}
	}
}

func TestBranchesInvalidScope(t *testing.T) {
	_, err := openRepo(t, seededBranches(t)).Branches(context.Background(), BranchScope(9))
	if ge, ok := err.(*GitError); !ok || ge.Code != CodeParseFailed {
		t.Fatalf("err = %v, want parse_failed", err)
	}
}

func TestCurrentBranch(t *testing.T) {
	ctx := context.Background()
	dir := seededBranches(t)
	r := openRepo(t, dir)
	name, err := r.currentBranch(ctx)
	if err != nil || name != "main" {
		t.Fatalf("currentBranch = %q, %v; want main", name, err)
	}
	gitOut(t, dir, "checkout", "-q", "--detach")
	name, err = r.currentBranch(ctx)
	if err != nil || name != "" {
		t.Fatalf("detached currentBranch = %q, %v; want empty", name, err)
	}
}

func TestCurrentBranchUnreadableHead(t *testing.T) {
	skipWithoutUnixPerms(t)
	r := openRepo(t, seededBranches(t))
	head := filepath.Join(r.path, ".git", "HEAD")
	if err := os.Chmod(head, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(head, 0o644) })
	_, err := r.currentBranch(context.Background())
	ge, ok := err.(*GitError)
	if !ok {
		t.Fatalf("err = %v, want GitError", err)
	}
	if ge.ExitCode == 1 {
		t.Fatalf("unreadable HEAD mapped to detached: %+v", ge)
	}
}
