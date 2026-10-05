package git

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func commitFile(t *testing.T, dir, name, content, msg string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"add", name}, {"commit", "-qm", msg}} {
		if out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
	}
}

func commitFileOnlyAdd(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command("git", "-C", dir, "add", name).CombinedOutput(); err != nil {
		t.Fatalf("git add: %v: %s", err, out)
	}
}

func gitRun(t *testing.T, dir string, args ...string) string {
	t.Helper()
	args = append([]string{"-C", dir}, args...)
	out, err := exec.Command("git", args...).CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v: %s", args, err, out)
	}
	return string(out)
}

func TestStatusCleanRepoReportsBranch(t *testing.T) {
	dir := initRepo(t)
	commitFile(t, dir, "a.txt", "a\n", "add a")

	repo, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	res, err := repo.Status(context.Background())
	if err != nil {
		t.Fatalf("Status: %v", err)
	}

	wantOid := gitRun(t, dir, "rev-parse", "HEAD")
	wantOid = wantOid[:len(wantOid)-1]
	if res.Branch.Head != "main" {
		t.Fatalf("Branch.Head = %q, want main", res.Branch.Head)
	}
	if res.Branch.Oid != wantOid {
		t.Fatalf("Branch.Oid = %q, want %q", res.Branch.Oid, wantOid)
	}
	if res.Branch.Upstream != "" || res.Branch.Ahead != 0 || res.Branch.Behind != 0 {
		t.Fatalf("upstream fields set without upstream: %+v", res.Branch)
	}
	if len(res.Files) != 0 {
		t.Fatalf("Files = %v, want empty", res.Files)
	}
}

func TestStatusReportsStagedAndUnstagedModifications(t *testing.T) {
	dir := initRepo(t)
	commitFile(t, dir, "a.txt", "a\n", "add a")
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("changed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	commitFileOnlyAdd(t, dir, "b.txt", "b\n")

	repo, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	res, err := repo.Status(context.Background())
	if err != nil {
		t.Fatalf("Status: %v", err)
	}

	byPath := map[string]FileStatus{}
	for _, f := range res.Files {
		byPath[f.Path] = f
	}
	if len(byPath) != 2 {
		t.Fatalf("files = %v, want a.txt and b.txt", res.Files)
	}
	a := byPath["a.txt"]
	if a.XY != ".M" || !a.Unstaged() || a.Staged() || a.Change() != ChangeModified {
		t.Fatalf("a.txt = %+v, want unstaged modified", a)
	}
	b := byPath["b.txt"]
	if b.XY != "A." || !b.Staged() || b.Unstaged() || b.Change() != ChangeAdded {
		t.Fatalf("b.txt = %+v, want staged added", b)
	}
}

func TestStatusReportsUntrackedWithQuestionRecord(t *testing.T) {
	dir := initRepo(t)
	commitFile(t, dir, "a.txt", "a\n", "add a")
	if err := os.WriteFile(filepath.Join(dir, "free.txt"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	repo, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	res, err := repo.Status(context.Background())
	if err != nil {
		t.Fatalf("Status: %v", err)
	}

	for _, f := range res.Files {
		if f.Path == "free.txt" {
			if !f.Untracked || f.Staged() || f.Unstaged() || f.Change() != ChangeUntracked {
				t.Fatalf("free.txt = %+v, want untracked", f)
			}
			return
		}
	}
	t.Fatalf("free.txt missing from %v", res.Files)
}

func TestStatusReportsRenameWithOriginalPath(t *testing.T) {
	dir := initRepo(t)
	commitFile(t, dir, "old name.txt", "content\n", "add")
	if out, err := exec.Command("git", "-C", dir, "mv", "old name.txt", "new name.txt").CombinedOutput(); err != nil {
		t.Fatalf("git mv: %v: %s", err, out)
	}

	repo, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	res, err := repo.Status(context.Background())
	if err != nil {
		t.Fatalf("Status: %v", err)
	}

	if len(res.Files) != 1 {
		t.Fatalf("files = %+v, want one rename", res.Files)
	}
	f := res.Files[0]
	if f.Path != "new name.txt" || f.OrigPath != "old name.txt" {
		t.Fatalf("paths = %q <- %q, want new name.txt <- old name.txt", f.Path, f.OrigPath)
	}
	if f.XY != "R." || f.Change() != ChangeRenamed || !f.Staged() {
		t.Fatalf("rename = %+v, want staged R.", f)
	}
}

func TestStatusReportsUnmergedConflict(t *testing.T) {
	dir := initRepo(t)
	commitFile(t, dir, "conflict.txt", "base\n", "base")
	if out, err := exec.Command("git", "-C", dir, "checkout", "-qb", "side").CombinedOutput(); err != nil {
		t.Fatalf("checkout: %v: %s", err, out)
	}
	commitFile(t, dir, "conflict.txt", "side\n", "side edit")
	if out, err := exec.Command("git", "-C", dir, "checkout", "-q", "main").CombinedOutput(); err != nil {
		t.Fatalf("checkout: %v: %s", err, out)
	}
	commitFile(t, dir, "conflict.txt", "main\n", "main edit")
	if out, err := exec.Command("git", "-C", dir, "merge", "side").CombinedOutput(); err == nil {
		t.Fatalf("merge should conflict: %s", out)
	}

	repo, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	res, err := repo.Status(context.Background())
	if err != nil {
		t.Fatalf("Status: %v", err)
	}

	if len(res.Files) != 1 {
		t.Fatalf("files = %+v, want one conflicted", res.Files)
	}
	f := res.Files[0]
	if f.Path != "conflict.txt" || f.XY != "UU" || !f.Conflict {
		t.Fatalf("conflict entry = %+v, want UU conflict.txt", f)
	}
	if f.Change() != ChangeConflicted {
		t.Fatalf("Change = %q, want conflicted", f.Change())
	}
}

func initBareRemote(t *testing.T, dir string) string {
	t.Helper()
	remote := filepath.Join(t.TempDir(), "remote.git")
	if out, err := exec.Command("git", "init", "--bare", "-q", remote).CombinedOutput(); err != nil {
		t.Fatalf("git init --bare: %v: %s", err, out)
	}
	if out, err := exec.Command("git", "-C", dir, "remote", "add", "origin", remote).CombinedOutput(); err != nil {
		t.Fatalf("git remote add: %v: %s", err, out)
	}
	if out, err := exec.Command("git", "-C", dir, "push", "-q", "-u", "origin", "main").CombinedOutput(); err != nil {
		t.Fatalf("git push: %v: %s", err, out)
	}
	return remote
}

func TestStatusReportsUpstreamAndAheadBehind(t *testing.T) {
	dir := initRepo(t)
	commitFile(t, dir, "a.txt", "a\n", "add a")
	initBareRemote(t, dir)
	commitFile(t, dir, "b.txt", "b\n", "add b")

	repo, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	res, err := repo.Status(context.Background())
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if res.Branch.Upstream != "origin/main" {
		t.Fatalf("Upstream = %q, want origin/main", res.Branch.Upstream)
	}
	if res.Branch.Ahead != 1 || res.Branch.Behind != 0 {
		t.Fatalf("ahead/behind = %d/%d, want 1/0", res.Branch.Ahead, res.Branch.Behind)
	}
}

func TestStatusReportsDetachedHead(t *testing.T) {
	dir := initRepo(t)
	commitFile(t, dir, "a.txt", "a\n", "first")
	commitFile(t, dir, "b.txt", "b\n", "second")
	if out, err := exec.Command("git", "-C", dir, "checkout", "-q", "--detach", "HEAD~1").CombinedOutput(); err != nil {
		t.Fatalf("checkout: %v: %s", err, out)
	}

	repo, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	res, err := repo.Status(context.Background())
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	want := strings.TrimSpace(gitRun(t, dir, "rev-parse", "HEAD"))
	if !res.Branch.Detached || res.Branch.Head != "" || res.Branch.Oid != want {
		t.Fatalf("branch = %+v, want detached at %s", res.Branch, want)
	}
}

func TestStatusReportsDeletedAndTypeChanged(t *testing.T) {
	dir := initRepo(t)
	commitFile(t, dir, "gone.txt", "x\n", "add gone")
	commitFile(t, dir, "exec.txt", "y\n", "add exec")
	if err := os.Remove(filepath.Join(dir, "gone.txt")); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(dir, "exec.txt")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("elsewhere", filepath.Join(dir, "exec.txt")); err != nil {
		t.Fatal(err)
	}

	repo, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	res, err := repo.Status(context.Background())
	if err != nil {
		t.Fatalf("Status: %v", err)
	}

	byPath := map[string]FileStatus{}
	for _, f := range res.Files {
		byPath[f.Path] = f
	}
	if d := byPath["gone.txt"]; d.XY != ".D" || d.Change() != ChangeDeleted || !d.Unstaged() {
		t.Fatalf("deleted = %+v, want unstaged .D", d)
	}
	if tc := byPath["exec.txt"]; tc.XY != ".T" || tc.Change() != ChangeTypeChanged {
		t.Fatalf("typechange = %+v, want .T", tc)
	}
}

func TestStatusFailsWhenRepoVanished(t *testing.T) {
	dir := initRepo(t)
	commitFile(t, dir, "a.txt", "a\n", "add a")
	repo, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(dir); err != nil {
		t.Fatal(err)
	}

	if _, err := repo.Status(context.Background()); err == nil {
		t.Fatal("want error for deleted work tree")
	}
}
