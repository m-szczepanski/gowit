package git

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// stashedRepo: one commit, then a tracked edit plus two untracked files
// (a.txt dirty, b.txt to be carried, c.txt to stay behind).
func stashedRepo(t *testing.T) string {
	t.Helper()
	dir := initRepo(t)
	writeFile(t, dir, "a.txt", "base\n")
	writeFile(t, dir, "keep.txt", "base\n")
	commitAll(t, dir, "base")
	writeFile(t, dir, "a.txt", "dirty\n")
	writeFile(t, dir, "b.txt", "untracked\n")
	return dir
}

func fileContent(t *testing.T, dir, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, name))
	if err != nil {
		if os.IsNotExist(err) {
			return ""
		}
		t.Fatal(err)
	}
	return string(data)
}

func stashCount(t *testing.T, dir string) string {
	t.Helper()
	return gitOut(t, dir, "stash", "list", "--format=%gs")
}

func TestStashPushTrackedAndUntracked(t *testing.T) {
	ctx := context.Background()
	dir := stashedRepo(t)
	r := openRepo(t, dir)

	if err := r.StashPush(ctx, StashPushOptions{}); err != nil {
		t.Fatalf("StashPush: %v", err)
	}

	if got := fileContent(t, dir, "a.txt"); got != "base\n" {
		t.Fatalf("tracked file after push = %q, want reverted to base", got)
	}
	if got := fileContent(t, dir, "b.txt"); got != "untracked\n" {
		t.Fatalf("tracked-only push must leave untracked b.txt alone, got %q", got)
	}
	if count := lines(t, stashCount(t, dir)); count != 1 || !strings.HasPrefix(stashCount(t, dir), "WIP on ") {
		t.Fatalf("stash list = %q, want one WIP entry", stashCount(t, dir))
	}

	if err := r.StashPush(ctx, StashPushOptions{IncludeUntracked: true}); err != nil {
		t.Fatalf("StashPush -u: %v", err)
	}
	if got := fileContent(t, dir, "b.txt"); got != "" {
		t.Fatalf("untracked b.txt after -u push = %q, want gone", got)
	}
	if count := lines(t, stashCount(t, dir)); count != 2 {
		t.Fatalf("stash list = %q, want two entries", stashCount(t, dir))
	}
}

func TestStashPushMessageAndPathScope(t *testing.T) {
	ctx := context.Background()
	dir := stashedRepo(t)
	writeFile(t, dir, "keep.txt", "also dirty\n")
	r := openRepo(t, dir)

	if err := r.StashPush(ctx, StashPushOptions{Message: "lunch plans", Paths: []string{"a.txt"}}); err != nil {
		t.Fatalf("StashPush scoped: %v", err)
	}
	if got := fileContent(t, dir, "a.txt"); got != "base\n" {
		t.Fatalf("scoped path a.txt = %q, want reverted", got)
	}
	if got := fileContent(t, dir, "keep.txt"); got != "also dirty\n" {
		t.Fatalf("out-of-scope keep.txt = %q, want still dirty", got)
	}
	if got := fileContent(t, dir, "b.txt"); got != "untracked\n" {
		t.Fatalf("untracked b.txt during scoped push = %q, want untouched", got)
	}
	subject := stashCount(t, dir)
	if !strings.HasPrefix(subject, "On main: lunch plans") {
		t.Fatalf("named stash subject = %q, want %q", subject, "On main: lunch plans")
	}
}

func TestStashPushNoChangesSucceedsWithoutEntry(t *testing.T) {
	ctx := context.Background()
	dir := initRepo(t)
	writeFile(t, dir, "a.txt", "x\n")
	commitAll(t, dir, "c")
	r := openRepo(t, dir)

	if err := r.StashPush(ctx, StashPushOptions{}); err != nil {
		t.Fatalf("StashPush on clean tree = %v, want nil (git no-ops)", err)
	}
	if out := gitOut(t, dir, "stash", "list"); out != "" {
		t.Fatalf("stash list after no-op push = %q, want empty", out)
	}
}

func lines(t *testing.T, s string) int {
	t.Helper()
	if s == "" {
		return 0
	}
	return len(strings.Split(strings.TrimSpace(s), "\n"))
}
