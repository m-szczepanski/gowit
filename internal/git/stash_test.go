package git

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
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

func mustPush(t *testing.T, r *Repo, opts StashPushOptions) {
	t.Helper()
	if err := r.StashPush(context.Background(), opts); err != nil {
		t.Fatalf("StashPush %+v: %v", opts, err)
	}
}

func TestStashApplyKeepsEntry(t *testing.T) {
	ctx := context.Background()
	dir := stashedRepo(t)
	r := openRepo(t, dir)
	mustPush(t, r, StashPushOptions{Message: "mine"})
	if got := fileContent(t, dir, "a.txt"); got != "base\n" {
		t.Fatalf("after push a.txt = %q", got)
	}
	if err := r.StashApply(ctx, 0); err != nil {
		t.Fatalf("StashApply: %v", err)
	}
	if got := fileContent(t, dir, "a.txt"); got != "dirty\n" {
		t.Fatalf("after apply a.txt = %q, want dirty", got)
	}
	list, err := r.StashList(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].Index != 0 {
		t.Fatalf("apply must keep the entry, list = %+v", list)
	}
}

func TestStashApplyOlderEntry(t *testing.T) {
	ctx := context.Background()
	dir := stashedRepo(t)
	r := openRepo(t, dir)
	mustPush(t, r, StashPushOptions{Message: "one"})
	writeFile(t, dir, "keep.txt", "two\n")
	mustPush(t, r, StashPushOptions{Message: "two"})
	if got := fileContent(t, dir, "keep.txt"); got != "base\n" {
		t.Fatalf("keep.txt after second push = %q", got)
	}
	if err := r.StashApply(ctx, 1); err != nil {
		t.Fatalf("StashApply(1): %v", err)
	}
	if got := fileContent(t, dir, "a.txt"); got != "dirty\n" {
		t.Fatalf("stash@{1} carried a.txt, got %q", got)
	}
	if got := fileContent(t, dir, "keep.txt"); got != "base\n" {
		t.Fatalf("older stash must not restore newer work, keep.txt = %q", got)
	}
	if err := r.StashApply(ctx, 0); err != nil {
		t.Fatalf("StashApply(0): %v", err)
	}
	if got := fileContent(t, dir, "keep.txt"); got != "two\n" {
		t.Fatalf("keep.txt after applying stash@{0} = %q, want two", got)
	}
}

func TestStashPopRemovesEntry(t *testing.T) {
	ctx := context.Background()
	dir := stashedRepo(t)
	r := openRepo(t, dir)
	mustPush(t, r, StashPushOptions{})
	if err := r.StashPop(ctx, 0); err != nil {
		t.Fatalf("StashPop: %v", err)
	}
	if got := fileContent(t, dir, "a.txt"); got != "dirty\n" {
		t.Fatalf("after pop a.txt = %q, want dirty", got)
	}
	list, err := r.StashList(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 0 {
		t.Fatalf("pop must remove the entry, list = %+v", list)
	}
}

func TestStashDropRemovesAndMissingIndexFails(t *testing.T) {
	ctx := context.Background()
	dir := stashedRepo(t)
	r := openRepo(t, dir)
	mustPush(t, r, StashPushOptions{})
	if err := r.StashDrop(ctx, 0); err != nil {
		t.Fatalf("StashDrop: %v", err)
	}
	list, _ := r.StashList(ctx)
	if len(list) != 0 {
		t.Fatalf("drop must remove the entry, list = %+v", list)
	}
	err := r.StashDrop(ctx, 0)
	if !errors.Is(err, ErrCommandFailed) {
		t.Fatalf("second drop err = %v, want command_failed", err)
	}
}

func TestStashNegativeIndex(t *testing.T) {
	ctx := context.Background()
	r := openRepo(t, stashedRepo(t))
	for _, op := range []func(context.Context, int) error{r.StashApply, r.StashPop, r.StashDrop} {
		if err := op(ctx, -1); !errors.Is(err, ErrValidationFailed) {
			t.Fatalf("err = %v, want validation_failed", err)
		}
	}
}

// stashConflictRepo: stash a change, then commit a conflicting change on
// the same line so restoring the stash cannot merge cleanly.
func stashConflictRepo(t *testing.T) *Repo {
	t.Helper()
	dir := initRepo(t)
	writeFile(t, dir, "a.txt", "base\n")
	commitAll(t, dir, "base")
	writeFile(t, dir, "a.txt", "stash-side\n")
	mustPush(t, openRepo(t, dir), StashPushOptions{Message: "mine"})
	writeFile(t, dir, "a.txt", "work-side\n")
	commitAll(t, dir, "work")
	return openRepo(t, dir)
}

func TestStashApplyConflictIsTypedAndKeepsState(t *testing.T) {
	ctx := context.Background()
	r := stashConflictRepo(t)
	err := r.StashApply(ctx, 0)
	if !errors.Is(err, ErrStashConflict) {
		t.Fatalf("err = %v, want stash_conflict", err)
	}
	list, listErr := r.StashList(ctx)
	if listErr != nil || len(list) != 1 {
		t.Fatalf("conflicted apply must keep the stash, list = %+v err = %v", list, listErr)
	}
	if !strings.Contains(fileContent(t, r.path, "a.txt"), "<<<<<<<") {
		t.Fatalf("conflict markers missing; work tree was rewritten or cleared")
	}
}

func TestStashPopConflictKeepsEntry(t *testing.T) {
	ctx := context.Background()
	r := stashConflictRepo(t)
	if err := r.StashPop(ctx, 0); !errors.Is(err, ErrStashConflict) {
		t.Fatalf("err = %v, want stash_conflict", err)
	}
	list, err := r.StashList(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 {
		t.Fatalf("conflicted pop must refuse to drop, list = %+v", list)
	}
}

func TestStashApplyUntrackedCollisionIsTyped(t *testing.T) {
	ctx := context.Background()
	dir := initRepo(t)
	writeFile(t, dir, "t.txt", "base\n")
	commitAll(t, dir, "base")
	writeFile(t, dir, "u.txt", "stashed untracked\n")
	mustPush(t, openRepo(t, dir), StashPushOptions{IncludeUntracked: true, Message: "u"})
	if got := fileContent(t, dir, "u.txt"); got != "" {
		t.Fatalf("untracked file should have left with the stash, got %q", got)
	}
	writeFile(t, dir, "u.txt", "new local\n")
	err := openRepo(t, dir).StashApply(ctx, 0)
	if !errors.Is(err, ErrStashConflict) {
		t.Fatalf("err = %v, want stash_conflict for untracked collision", err)
	}
}

func TestStashClearRemovesAll(t *testing.T) {
	ctx := context.Background()
	dir := stashedRepo(t)
	r := openRepo(t, dir)
	mustPush(t, r, StashPushOptions{Message: "one"})
	writeFile(t, dir, "keep.txt", "two\n")
	mustPush(t, r, StashPushOptions{Message: "two"})

	if err := r.StashClear(ctx); err != nil {
		t.Fatalf("StashClear: %v", err)
	}
	list, err := r.StashList(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 0 {
		t.Fatalf("clear must empty the list, got %+v", list)
	}
	// the work stays untouched: only the stash refs die
	if got := fileContent(t, dir, "a.txt"); got != "base\n" {
		t.Fatalf("a.txt = %q, clear must not restore anything", got)
	}
}

func TestStashListCtxKill(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := openRepo(t, stashedRepo(t)).StashList(ctx); !errors.Is(err, ErrTimeout) {
		t.Fatalf("err = %v, want timeout", err)
	}
}

func TestParseStashListSelectorJunk(t *testing.T) {
	if _, err := parseStashList("stash@{x}\x00On main: m\x001700000000"); err == nil {
		t.Fatal("want parse failure for non-numeric selector")
	}
}

func TestParseStashListNormal(t *testing.T) {
	in := "stash@{0}\x00On main: named\x001700000000\n" +
		"stash@{1}\x00WIP on main: abc def\x001800000000\n" +
		"stash@{2}\x00hand-stored mystery\x001900000000\n"
	list, err := parseStashList(in)
	if err != nil {
		t.Fatal(err)
	}
	want := []StashEntry{
		{Index: 0, Message: "On main: named", Type: "message", Date: time.Unix(1700000000, 0).UTC()},
		{Index: 1, Message: "WIP on main: abc def", Type: "wip", Date: time.Unix(1800000000, 0).UTC()},
		{Index: 2, Message: "hand-stored mystery", Type: "other", Date: time.Unix(1900000000, 0).UTC()},
	}
	if !reflect.DeepEqual(list, want) {
		t.Fatalf("got %+v, want %+v", list, want)
	}
}
