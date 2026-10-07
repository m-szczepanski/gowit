package git

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStageAddsGivenPathOnly(t *testing.T) {
	dir := initRepo(t)
	commitFile(t, dir, "a.txt", "a\n", "add a")
	commitFile(t, dir, "b.txt", "b\n", "add b")
	writeFile(t, dir, "a.txt", "changed\n")
	writeFile(t, dir, "b.txt", "also changed\n")

	repo, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.Stage(context.Background(), "a.txt"); err != nil {
		t.Fatalf("Stage: %v", err)
	}

	m := statusByPath(openStatus(t, dir))
	if f := m["a.txt"]; f.XY != "M." {
		t.Fatalf("a.txt = %+v, want staged M.", f)
	}
	if f := m["b.txt"]; f.XY != ".M" {
		t.Fatalf("b.txt = %+v, want unstaged .M", f)
	}
}

func TestStageHandlesSpacesAndUnicodePaths(t *testing.T) {
	dir := initRepo(t)
	commitFile(t, dir, "two words.txt", "a\n", "add")
	commitFile(t, dir, "żółw.png", "b\n", "add unicode")
	writeFile(t, dir, "two words.txt", "c\n")
	writeFile(t, dir, "żółw.png", "d\n")

	repo, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.Stage(context.Background(), "two words.txt", "żółw.png"); err != nil {
		t.Fatalf("Stage: %v", err)
	}

	m := statusByPath(openStatus(t, dir))
	if m["two words.txt"].XY != "M." || m["żółw.png"].XY != "M." {
		t.Fatalf("nothing staged: %+v", m)
	}
}

func TestStageReportsMissingPathspec(t *testing.T) {
	dir := initRepo(t)
	commitFile(t, dir, "a.txt", "a\n", "add a")

	repo, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	err = repo.Stage(context.Background(), "nosuch.txt")
	if !errors.Is(err, ErrCommandFailed) {
		t.Fatalf("err = %v, want command_failed GitError", err)
	}
	if !strings.Contains(err.Error(), "nosuch.txt") {
		t.Fatalf("error should name the bad path: %v", err)
	}
}

func TestUnstageKeepsWorktreeChanges(t *testing.T) {
	dir := initRepo(t)
	commitFile(t, dir, "a.txt", "a\n", "add a")
	writeFile(t, dir, "a.txt", "edited\n")
	addFile(t, dir, "new.txt", "fresh\n")

	repo, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.Unstage(context.Background(), "a.txt", "new.txt"); err != nil {
		t.Fatalf("Unstage: %v", err)
	}

	m := statusByPath(openStatus(t, dir))
	if f := m["a.txt"]; f.XY != ".M" || f.Staged {
		t.Fatalf("a.txt = %+v, want unstaged modified", f)
	}
	if f := m["new.txt"]; !f.Untracked {
		t.Fatalf("new.txt = %+v, want untracked after unstaging a never-committed file", f)
	}
	content, err := os.ReadFile(filepath.Join(dir, "a.txt"))
	if err != nil || string(content) != "edited\n" {
		t.Fatalf("worktree edit lost: %q %v", content, err)
	}
}

func TestStageAndUnstageRequirePaths(t *testing.T) {
	dir := initRepo(t)
	commitFile(t, dir, "a.txt", "a\n", "add a")
	repo, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()

	// `git add --` with no pathspec succeeds while doing nothing, and
	// reset/restore without one would touch the whole work tree
	for _, err := range []error{repo.Stage(ctx), repo.Unstage(ctx), repo.DiscardChanges(ctx)} {
		var ge *GitError
		if !errors.As(err, &ge) || !errors.Is(err, ErrValidationFailed) {
			t.Fatalf("empty path call err = %v, want validation_failed", err)
		}
		if !strings.Contains(ge.Message, "path") {
			t.Fatalf("message = %q, should mention paths", ge.Message)
		}
	}
}

func TestStageAllAndUnstageAll(t *testing.T) {
	dir := initRepo(t)
	commitFile(t, dir, "a.txt", "a\n", "add a")
	commitFile(t, dir, "gone.txt", "g\n", "add gone")
	writeFile(t, dir, "a.txt", "edited\n")
	writeFile(t, dir, "new.txt", "n\n")
	if err := os.Remove(filepath.Join(dir, "gone.txt")); err != nil {
		t.Fatal(err)
	}

	repo, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if err := repo.StageAll(ctx); err != nil {
		t.Fatalf("StageAll: %v", err)
	}

	m := statusByPath(openStatus(t, dir))
	if m["a.txt"].XY != "M." || !m["new.txt"].Staged || m["gone.txt"].XY != "D." {
		t.Fatalf("after StageAll = %+v", m)
	}

	if err := repo.UnstageAll(ctx); err != nil {
		t.Fatalf("UnstageAll: %v", err)
	}
	m = statusByPath(openStatus(t, dir))
	for _, f := range m {
		if f.Staged {
			t.Fatalf("%s still staged after UnstageAll: %+v", f.Path, f)
		}
	}
}

func TestDiscardChangesRevertsTrackedAndRemovesUntracked(t *testing.T) {
	dir := initRepo(t)
	commitFile(t, dir, "a.txt", "original\n", "add a")
	writeFile(t, dir, "a.txt", "ruined\n")
	writeFile(t, dir, "scratch.txt", "temp\n")

	repo, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.DiscardChanges(context.Background(), "a.txt", "scratch.txt"); err != nil {
		t.Fatalf("DiscardChanges: %v", err)
	}

	if content, err := os.ReadFile(filepath.Join(dir, "a.txt")); err != nil || string(content) != "original\n" {
		t.Fatalf("tracked file not restored: %q %v", content, err)
	}
	if _, err := os.Stat(filepath.Join(dir, "scratch.txt")); !os.IsNotExist(err) {
		t.Fatalf("untracked file should be deleted, stat: %v", err)
	}
	if res := openStatus(t, dir); len(res.Files) != 0 {
		t.Fatalf("status after discard = %v, want clean", res.Files)
	}
}

func TestDiscardChangesFailsLoudlyOnIgnoredAndUnknown(t *testing.T) {
	dir := initRepo(t)
	commitFile(t, dir, ".gitignore", "secret.pem\n", "ignore secrets")
	writeFile(t, dir, "secret.pem", "private\n")

	repo, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if err := repo.DiscardChanges(ctx, "secret.pem"); !errors.Is(err, ErrCommandFailed) {
		t.Fatalf("ignored path err = %v, want loud command_failed", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "secret.pem")); err != nil {
		t.Fatalf("ignored file must survive: %v", err)
	}
	if err := repo.DiscardChanges(ctx, "nosuch.txt"); !errors.Is(err, ErrCommandFailed) {
		t.Fatalf("unknown path err = %v, want command_failed", err)
	}
}

func TestDiscardChangesUntrackedOnly(t *testing.T) {
	dir := initRepo(t)
	commitFile(t, dir, "a.txt", "a\n", "add a")
	writeFile(t, dir, "scratch.txt", "temp\n")

	repo, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.DiscardChanges(context.Background(), "scratch.txt"); err != nil {
		t.Fatalf("DiscardChanges: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "scratch.txt")); !os.IsNotExist(err) {
		t.Fatalf("untracked file should be gone, stat: %v", err)
	}
}

func TestDiscardChangesFailsWhenRepoVanished(t *testing.T) {
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
		t.Fatal("precondition: Status should fail")
	}
	err = repo.DiscardChanges(context.Background(), "a.txt")
	if !errors.Is(err, ErrCommandFailed) {
		t.Fatalf("err = %v, want command_failed GitError", err)
	}
}

func TestDiscardLeavesStagedChangesAlone(t *testing.T) {
	dir := initRepo(t)
	commitFile(t, dir, "a.txt", "head\n", "add a")
	writeFile(t, dir, "a.txt", "staged\n")
	addFile(t, dir, "a.txt", "staged\n")
	writeFile(t, dir, "a.txt", "staged+worktree\n")
	if f := statusByPath(openStatus(t, dir))["a.txt"]; f.XY != "MM" {
		t.Fatalf("precondition: a.txt = %+v, want MM", f)
	}

	repo, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.DiscardChanges(context.Background(), "a.txt"); err != nil {
		t.Fatalf("DiscardChanges: %v", err)
	}

	if f := statusByPath(openStatus(t, dir))["a.txt"]; f.XY != "M." {
		t.Fatalf("a.txt = %+v, want worktree edit reverted, staged content kept (M.)", f)
	}
	content, err := os.ReadFile(filepath.Join(dir, "a.txt"))
	if err != nil || string(content) != "staged\n" {
		t.Fatalf("work tree not restored to index: %q %v", content, err)
	}
}

func TestDiscardRemovesUnicodeUntracked(t *testing.T) {
	dir := initRepo(t)
	commitFile(t, dir, "a.txt", "a\n", "add a")
	writeFile(t, dir, "żółw śmieć.tmp", "temp\n")

	repo, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.DiscardChanges(context.Background(), "żółw śmieć.tmp"); err != nil {
		t.Fatalf("DiscardChanges: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "żółw śmieć.tmp")); !os.IsNotExist(err) {
		t.Fatalf("unicode untracked file survived: %v", err)
	}
}

func TestDiscardUntrackedNestedAndCollapsedDir(t *testing.T) {
	dir := initRepo(t)
	commitFile(t, dir, "a.txt", "head\n", "add a")
	if err := os.Mkdir(filepath.Join(dir, "build"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, dir, "build/sub.txt", "x\n")
	writeFile(t, dir, "build/deep.txt", "y\n")
	writeFile(t, dir, "a.txt", "ruined\n")

	repo, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()

	// status collapses the folder to "build/", but a file inside it must
	// still be discardable, alongside a tracked edit in the same call
	if err := repo.DiscardChanges(ctx, "a.txt", "build/sub.txt"); err != nil {
		t.Fatalf("DiscardChanges nested path: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "build/sub.txt")); !os.IsNotExist(err) {
		t.Fatalf("build/sub.txt survived: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "build/deep.txt")); err != nil {
		t.Fatalf("sibling should remain: %v", err)
	}
	content, err := os.ReadFile(filepath.Join(dir, "a.txt"))
	if err != nil || string(content) != "head\n" {
		t.Fatalf("tracked edit not reverted: %q %v", content, err)
	}

	// checking the collapsed directory entry discards the whole folder
	if err := repo.DiscardChanges(ctx, "build/"); err != nil {
		t.Fatalf("DiscardChanges build/: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "build")); !os.IsNotExist(err) {
		t.Fatal("build/ should be gone")
	}
}

func TestDiscardAbortsBeforeDeletingWhenTrackedPathBad(t *testing.T) {
	dir := initRepo(t)
	commitFile(t, dir, "a.txt", "a\n", "add a")
	writeFile(t, dir, "scratch.txt", "temp\n")

	repo, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.DiscardChanges(context.Background(), "nosuch.txt", "scratch.txt"); !errors.Is(err, ErrCommandFailed) {
		t.Fatalf("err = %v, want command_failed", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "scratch.txt")); err != nil {
		t.Fatalf("nothing may be deleted when validation fails: %v", err)
	}
}
