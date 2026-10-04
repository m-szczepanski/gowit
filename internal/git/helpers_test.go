package git

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func commitAll(t *testing.T, dir, msg string) {
	t.Helper()
	ctx := context.Background()
	if _, _, err := runGit(ctx, dir, "add", "-A"); err != nil {
		t.Fatal(err)
	}
	if _, _, err := runGit(ctx, dir, "-c", "user.email=t@t", "-c", "user.name=t", "commit", "-m", msg); err != nil {
		t.Fatal(err)
	}
}

func currentBranch(t *testing.T, dir string) string {
	t.Helper()
	out, _, err := runGit(context.Background(), dir, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	return strings.TrimSpace(string(out))
}

// conflictingRepo: base commit, feature and base both editing a.txt differently.
func conflictingRepo(t *testing.T) string {
	t.Helper()
	dir := initRepo(t)
	ctx := context.Background()
	writeFile(t, dir, "a.txt", "one\n")
	commitAll(t, dir, "base")
	base := currentBranch(t, dir)

	if _, _, err := runGit(ctx, dir, "checkout", "-qb", "feature"); err != nil {
		t.Fatal(err)
	}
	writeFile(t, dir, "a.txt", "feature line\n")
	commitAll(t, dir, "feature change")

	if _, _, err := runGit(ctx, dir, "checkout", "-q", base); err != nil {
		t.Fatal(err)
	}
	writeFile(t, dir, "a.txt", "base line\n")
	commitAll(t, dir, "base change")
	return dir
}
