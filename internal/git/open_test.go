package git

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestOpenResolvesSubdirectoryToToplevel(t *testing.T) {
	dir := initRepo(t)
	sub := filepath.Join(dir, "internal", "git")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}

	repo, err := Open(sub)
	if err != nil {
		t.Fatalf("Open(subdir): %v", err)
	}

	want, err := filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatal(err)
	}
	got, err := filepath.EvalSymlinks(repo.Path())
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("Path() = %q, want toplevel %q", got, want)
	}
}

func TestOpenRejectsBareRepository(t *testing.T) {
	dir := t.TempDir()
	out, err := exec.Command("git", "init", "--bare", dir).CombinedOutput()
	if err != nil {
		t.Fatalf("git init --bare: %v: %s", err, out)
	}

	_, err = Open(dir)
	if !errors.Is(err, ErrBareRepository) {
		t.Fatalf("Open(bare) err = %v, want ErrBareRepository", err)
	}
	if !strings.Contains(err.Error(), dir) {
		t.Fatalf("error %q should name the offending path", err)
	}
}

func TestOpenParentOfRepositoriesSuggestsChildren(t *testing.T) {
	parent := t.TempDir()
	for _, name := range []string{"alpha", "beta"} {
		child := filepath.Join(parent, name)
		if err := os.Mkdir(child, 0o755); err != nil {
			t.Fatal(err)
		}
		if out, err := exec.Command("git", "init", child).CombinedOutput(); err != nil {
			t.Fatalf("git init %s: %v: %s", name, err, out)
		}
	}
	if err := os.WriteFile(filepath.Join(parent, "notes.txt"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := Open(parent)
	if !errors.Is(err, ErrNotARepository) {
		t.Fatalf("Open(parent) err = %v, want ErrNotARepository", err)
	}
	msg := err.Error()
	if !strings.Contains(msg, "alpha") || !strings.Contains(msg, "beta") {
		t.Fatalf("error should list the child repos, got %q", msg)
	}
	if strings.Contains(msg, "notes.txt") {
		t.Fatalf("plain files must not be suggested as repos: %q", msg)
	}
}

func TestOpenParentOfSingleRepositoryUsesSingularGrammar(t *testing.T) {
	parent := t.TempDir()
	solo := filepath.Join(parent, "solo")
	if err := os.Mkdir(solo, 0o755); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command("git", "init", solo).CombinedOutput(); err != nil {
		t.Fatalf("git init: %v: %s", err, out)
	}

	_, err := Open(parent)
	if err == nil {
		t.Fatal("want error")
	}
	msg := err.Error()
	if !strings.Contains(msg, "solo") || !strings.Contains(msg, "is - open it") {
		t.Fatalf("singular hint expected, got %q", msg)
	}
	if strings.Contains(msg, "are -") {
		t.Fatalf("plural grammar used for a single child: %q", msg)
	}
}

func TestOpenPlainDirectoryDoesNotClaimChildren(t *testing.T) {
	_, err := Open(t.TempDir())
	if !errors.Is(err, ErrNotARepository) {
		t.Fatalf("err = %v, want ErrNotARepository", err)
	}
	if strings.Contains(err.Error(), "contains") {
		t.Fatalf("hint must not appear when there are no child repos: %q", err)
	}
}
