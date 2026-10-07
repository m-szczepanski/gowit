package git

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// setGitIdentity pins the committer identity in a repo created by git
// clone: clone does not carry the source's local user config, and CI
// runners have no global identity to fall back on.
func setGitIdentity(t *testing.T, dir string) {
	t.Helper()
	for _, kv := range [][2]string{{"user.email", "t@t"}, {"user.name", "test"}} {
		if _, _, err := runGit(context.Background(), dir, "config", kv[0], kv[1]); err != nil {
			t.Fatalf("git config %s: %v", kv[0], err)
		}
	}
}

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
	if _, _, err := runGit(ctx, dir, "commit", "-m", msg); err != nil {
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

func openStatus(t *testing.T, dir string) *StatusResult {
	t.Helper()
	repo, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	res, err := repo.Status(context.Background())
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	return res
}

func statusByPath(res *StatusResult) map[string]FileStatus {
	m := map[string]FileStatus{}
	for _, f := range res.Files {
		m[f.Path] = f
	}
	return m
}

// unixPerms reports whether the platform maps chmod onto the git exec bit.
// Windows: core.fileMode is off and Go's Mode() carries no 0111 bits.
func unixPerms() bool { return runtime.GOOS != "windows" }

func skipWithoutUnixPerms(t *testing.T) {
	t.Helper()
	if !unixPerms() {
		t.Skip("no exec bit on Windows; git ignores chmod there")
	}
}
