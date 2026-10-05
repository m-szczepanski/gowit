package git

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func commitFile(t *testing.T, dir, name, content, msg string) {
	t.Helper()
	writeFile(t, dir, name, content)
	commitAll(t, dir, msg)
}

func addFile(t *testing.T, dir, name, content string) {
	t.Helper()
	writeFile(t, dir, name, content)
	if _, _, err := runGit(context.Background(), dir, "add", name); err != nil {
		t.Fatalf("git add %s: %v", name, err)
	}
}

func mustGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	out, _, err := runGit(context.Background(), dir, args...)
	if err != nil {
		t.Fatalf("git %v: %v", args, err)
	}
	return strings.TrimSpace(string(out))
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

	if res.Branch.Head != "main" {
		t.Fatalf("Branch.Head = %q, want main", res.Branch.Head)
	}
	if want := mustGit(t, dir, "rev-parse", "HEAD"); res.Branch.Oid != want {
		t.Fatalf("Branch.Oid = %q, want %q", res.Branch.Oid, want)
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
	commitFile(t, dir, "c.txt", "c\n", "add c")
	writeFile(t, dir, "a.txt", "changed\n")
	writeFile(t, dir, "c.txt", "staged change\n")
	if _, _, err := runGit(context.Background(), dir, "add", "c.txt"); err != nil {
		t.Fatal(err)
	}
	addFile(t, dir, "b.txt", "b\n")

	repo, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	res, err := repo.Status(context.Background())
	if err != nil {
		t.Fatalf("Status: %v", err)
	}

	byPath := statusByPath(res)
	if len(byPath) != 3 {
		t.Fatalf("files = %+v, want a.txt b.txt c.txt", res.Files)
	}
	if a := byPath["a.txt"]; a.XY != ".M" || !a.Unstaged || a.Staged || a.Change != ChangeModified {
		t.Fatalf("a.txt = %+v, want unstaged modified", a)
	}
	if c := byPath["c.txt"]; c.XY != "M." || !c.Staged || c.Unstaged || c.Change != ChangeModified {
		t.Fatalf("c.txt = %+v, want staged modified", c)
	}
	if b := byPath["b.txt"]; b.XY != "A." || !b.Staged || b.Change != ChangeAdded {
		t.Fatalf("b.txt = %+v, want staged added", b)
	}
}

func TestStatusReportsUntrackedWithQuestionRecord(t *testing.T) {
	dir := initRepo(t)
	commitFile(t, dir, "a.txt", "a\n", "add a")
	writeFile(t, dir, "free.txt", "x\n")

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
			if !f.Untracked || f.Staged || f.Unstaged || f.Change != ChangeUntracked {
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
	if _, _, err := runGit(context.Background(), dir, "mv", "old name.txt", "new name.txt"); err != nil {
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

	if len(res.Files) != 1 {
		t.Fatalf("files = %+v, want one rename", res.Files)
	}
	f := res.Files[0]
	if f.Path != "new name.txt" || f.OrigPath != "old name.txt" {
		t.Fatalf("paths = %q <- %q, want new name.txt <- old name.txt", f.Path, f.OrigPath)
	}
	if f.XY != "R." || f.Change != ChangeRenamed || !f.Staged {
		t.Fatalf("rename = %+v, want staged R.", f)
	}
}

func TestStatusReportsUnmergedConflictWithStages(t *testing.T) {
	dir := conflictingRepo(t)
	if _, _, err := runGit(context.Background(), dir, "merge", "feature"); err == nil {
		t.Fatal("merge should conflict")
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
	if f.Path != "a.txt" || f.XY != "UU" || !f.Conflict || f.Change != ChangeConflicted {
		t.Fatalf("conflict entry = %+v, want UU a.txt", f)
	}

	// independent truth: git ls-files -u prints "<mode> <oid> <stage>\t<path>"
	want := map[int]string{}
	for _, line := range strings.Split(mustGit(t, dir, "ls-files", "-u"), "\n") {
		fields := strings.Fields(strings.SplitN(line, "\t", 2)[0])
		stage, err := strconv.Atoi(fields[2])
		if err != nil {
			t.Fatal(err)
		}
		want[stage] = fields[0] + " " + fields[1]
	}
	if len(f.Stages) != 3 {
		t.Fatalf("stages = %+v, want three positions", f.Stages)
	}
	for _, st := range f.Stages {
		got := st.Mode + " " + st.Oid
		if want[st.Stage] != got {
			t.Fatalf("stage %d = %q, want %q", st.Stage, got, want[st.Stage])
		}
	}
}

func TestStatusReportsUpstreamAheadAndBehind(t *testing.T) {
	dir := initRepo(t)
	commitFile(t, dir, "a.txt", "a\n", "add a")
	remote := filepath.Join(t.TempDir(), "remote.git")
	ctx := context.Background()
	if _, _, err := runGit(ctx, dir, "-c", "init.defaultBranch=main", "init", "--bare", "-q", remote); err != nil {
		t.Fatal(err)
	}
	if _, _, err := runGit(ctx, dir, "remote", "add", "origin", remote); err != nil {
		t.Fatal(err)
	}
	if _, _, err := runGit(ctx, dir, "push", "-q", "-u", "origin", "main"); err != nil {
		t.Fatal(err)
	}
	commitFile(t, dir, "b.txt", "b\n", "local ahead")

	// second clone pushes too, so the branch ends up both ahead and behind
	other := t.TempDir()
	if out, err := exec.Command("git", "clone", "-q", remote, other).CombinedOutput(); err != nil {
		t.Fatalf("clone: %v: %s", err, out)
	}
	// repo-local identity is not cloned; CI runners have no global one
	for _, kv := range [][2]string{{"user.email", "t@t"}, {"user.name", "test"}} {
		if _, _, err := runGit(ctx, other, "config", kv[0], kv[1]); err != nil {
			t.Fatal(err)
		}
	}
	commitFile(t, other, "c.txt", "c\n", "remote ahead")
	if _, _, err := runGit(ctx, other, "push", "-q", "origin", "HEAD:main"); err != nil {
		t.Fatal(err)
	}
	if _, _, err := runGit(ctx, dir, "fetch", "-q"); err != nil {
		t.Fatal(err)
	}

	repo, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	res, err := repo.Status(ctx)
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if res.Branch.Upstream != "origin/main" {
		t.Fatalf("Upstream = %q, want origin/main", res.Branch.Upstream)
	}
	if res.Branch.Ahead != 1 || res.Branch.Behind != 1 {
		t.Fatalf("ahead/behind = %d/%d, want 1/1", res.Branch.Ahead, res.Branch.Behind)
	}
}

func TestStatusReportsDetachedHead(t *testing.T) {
	dir := initRepo(t)
	commitFile(t, dir, "a.txt", "a\n", "first")
	commitFile(t, dir, "b.txt", "b\n", "second")
	if _, _, err := runGit(context.Background(), dir, "checkout", "-q", "--detach", "HEAD~1"); err != nil {
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
	if want := mustGit(t, dir, "rev-parse", "HEAD"); !res.Branch.Detached || res.Branch.Head != "" || res.Branch.Oid != want {
		t.Fatalf("branch = %+v, want detached at %s", res.Branch, want)
	}
}

func TestStatusReportsDeleted(t *testing.T) {
	dir := initRepo(t)
	commitFile(t, dir, "gone.txt", "x\n", "add gone")
	if err := os.Remove(filepath.Join(dir, "gone.txt")); err != nil {
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

	byPath := statusByPath(res)
	if d := byPath["gone.txt"]; d.XY != ".D" || d.Change != ChangeDeleted || !d.Unstaged {
		t.Fatalf("deleted = %+v, want unstaged .D", d)
	}
}

func TestStatusReportsTypeChanged(t *testing.T) {
	dir := initRepo(t)
	commitFile(t, dir, "link.txt", "y\n", "add link")
	if err := os.Remove(filepath.Join(dir, "link.txt")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("elsewhere", filepath.Join(dir, "link.txt")); err != nil {
		t.Skipf("symlinks unsupported: %v", err)
	}
	// Git for Windows defaults core.symlinks=false and materializes the
	// link as a regular file, so git reports .M instead of .T
	if out, _, err := runGit(context.Background(), dir, "config", "--bool", "core.symlinks"); err == nil && strings.TrimSpace(string(out)) == "false" {
		t.Skip("git core.symlinks=false")
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
		if f.Path == "link.txt" {
			if f.XY != ".T" || f.Change != ChangeTypeChanged {
				t.Fatalf("typechange = %+v, want .T", f)
			}
			return
		}
	}
	t.Fatal("link.txt missing from status")
}

func TestStatusUnbornBranchHasNoOid(t *testing.T) {
	dir := initRepo(t)

	repo, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	res, err := repo.Status(context.Background())
	if err != nil {
		t.Fatalf("Status: %v", err)
	}

	if res.Branch.Head != "main" || res.Branch.Oid != "" || res.Branch.Detached {
		t.Fatalf("branch = %+v, want main with empty oid on unborn branch", res.Branch)
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
