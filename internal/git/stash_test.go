package git

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
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

func TestStashListOrderAndFields(t *testing.T) {
	ctx := context.Background()
	dir := stashedRepo(t)
	r := openRepo(t, dir)
	if err := r.StashPush(ctx, StashPushOptions{}); err != nil {
		t.Fatal(err)
	}
	writeFile(t, dir, "a.txt", "again\n")
	if err := r.StashPush(ctx, StashPushOptions{Message: "second"}); err != nil {
		t.Fatal(err)
	}

	list, err := r.StashList(ctx)
	if err != nil {
		t.Fatalf("StashList: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("len = %d, want 2: %+v", len(list), list)
	}

	newest, older := list[0], list[1]
	if newest.Index != 0 || older.Index != 1 {
		t.Fatalf("indexes = %d,%d, want 0,1 newest first", newest.Index, older.Index)
	}
	if newest.Message != "On main: second" {
		t.Fatalf("newest Message = %q, want %q", newest.Message, "On main: second")
	}
	if newest.Type != "message" {
		t.Fatalf("newest Type = %q, want message", newest.Type)
	}
	if !strings.HasPrefix(older.Message, "WIP on main: ") || older.Type != "wip" {
		t.Fatalf("older = %+v, want WIP entry", older)
	}

	rawDates := gitOut(t, dir, "stash", "list", "--format=%ct")
	dateLines := strings.Split(rawDates, "\n")
	for i, line := range dateLines {
		sec, err := strconv.ParseInt(line, 10, 64)
		if err != nil {
			t.Fatal(err)
		}
		if want := time.Unix(sec, 0).UTC(); !list[i].Date.Equal(want) {
			t.Fatalf("entry %d Date = %v, want %v", i, list[i].Date, want)
		}
	}
}

func TestStashListEmpty(t *testing.T) {
	list, err := openRepo(t, stashedRepo(t)).StashList(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 0 {
		t.Fatalf("got %+v, want none", list)
	}
}

func TestParseStashListMalformed(t *testing.T) {
	cases := []string{
		"stash@{0}\x00On main: msg",
		"refs/weird\x00On main: msg\x001700000000",
		"stash@{0}\x00On main: msg\x00nan",
	}
	for _, in := range cases {
		if _, err := parseStashList(in); err == nil {
			t.Fatalf("parseStashList(%q) = nil, want parse failure", in)
		} else if ge, ok := err.(*GitError); !ok || ge.Code != CodeParseFailed {
			t.Fatalf("parseStashList(%q) err = %v, want parse_failed", in, err)
		}
	}
}
