package git

import (
	"errors"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func initRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	cmd := exec.Command("git", "init", dir)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init failed: %v: %s", err, out)
	}
	// CI runners have no global git identity; merge/commit would abort with
	// "Committer identity unknown" regardless of what the test intends
	for _, kv := range [][2]string{{"user.email", "t@t"}, {"user.name", "test"}} {
		if out, err := exec.Command("git", "-C", dir, "config", kv[0], kv[1]).CombinedOutput(); err != nil {
			t.Fatalf("git config %s: %v: %s", kv[0], err, out)
		}
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
	_, err := Open(missing)
	if !errors.Is(err, ErrPathMissing) {
		t.Fatalf("Open(%q): err = %v, want ErrPathMissing", missing, err)
	}
}

func TestOpenWithoutGitOnPath(t *testing.T) {
	dir := initRepo(t)
	t.Setenv("PATH", "")

	_, err := Open(dir)
	if err == nil {
		t.Fatal("Open on a real repo without git on PATH: want error, got nil")
	}
	if want := "git executable not found on PATH"; !strings.Contains(err.Error(), want) {
		t.Fatalf("Open error = %q, want it to mention %q", err, want)
	}
}
