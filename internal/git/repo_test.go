package git

import (
	"os/exec"
	"path/filepath"
	"testing"
)

func initRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	cmd := exec.Command("git", "init", dir)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init failed: %v: %s", err, out)
	}
	return dir
}

func TestOpenSucceedsOnGitWorkTree(t *testing.T) {
	dir := initRepo(t)

	repo, err := Open(dir)
	if err != nil {
		t.Fatalf("Open(%q): %v", dir, err)
	}

	// git may resolve symlinks (e.g. /tmp -> /private/tmp on macOS),
	// so compare resolved paths rather than raw strings.
	want, err := filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatal(err)
	}
	got, err := filepath.EvalSymlinks(repo.Path())
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("Path() = %q, want %q", got, want)
	}
	if err := repo.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

func TestOpenRejectsPlainDirectory(t *testing.T) {
	if _, err := Open(t.TempDir()); err == nil {
		t.Fatal("Open on non-repo directory: want error, got nil")
	}
}

func TestOpenRejectsMissingDirectory(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "nope")
	if _, err := Open(missing); err == nil {
		t.Fatalf("Open(%q): want error, got nil", missing)
	}
}

func TestOpenWithoutGitOnPath(t *testing.T) {
	t.Setenv("PATH", "")
	if _, err := Open("/tmp"); err == nil {
		t.Fatal("Open without git on PATH: want error, got nil")
	}
}
