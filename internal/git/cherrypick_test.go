package git

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// featureRepo: main at base; side branch carries two commits touching
// side.txt. Returns dir and the side branch's commit oids oldest first.
func featureRepo(t *testing.T) (string, []string) {
	t.Helper()
	dir := initRepo(t)
	writeFile(t, dir, "base.txt", "base\n")
	commitAll(t, dir, "base")
	gitOut(t, dir, "checkout", "-qb", "side")
	writeFile(t, dir, "side.txt", "one\n")
	commitAll(t, dir, "first pick")
	writeFile(t, dir, "side2.txt", "two\n")
	commitAll(t, dir, "second pick")
	gitOut(t, dir, "checkout", "-q", "main")
	oids := []string{
		gitOut(t, dir, "rev-parse", "side~1"),
		gitOut(t, dir, "rev-parse", "side"),
	}
	return dir, oids
}

func TestCherryPickSingle(t *testing.T) {
	ctx := context.Background()
	dir, oids := featureRepo(t)
	r := openRepo(t, dir)
	if err := r.CherryPick(ctx, []string{oids[0]}, CherryPickOptions{}); err != nil {
		t.Fatalf("CherryPick: %v", err)
	}
	if got := fileContent(t, dir, "side.txt"); got != "one\n" {
		t.Fatalf("side.txt = %q, want one", got)
	}
	subject := gitOut(t, dir, "log", "-1", "--format=%s")
	if subject != "first pick" {
		t.Fatalf("subject = %q, want first pick", subject)
	}
	if strings.Contains(gitOut(t, dir, "log", "-1", "--format=%B"), "cherry picked from") {
		t.Fatal("without -x the provenance trailer must be absent")
	}
}

func TestCherryPickRangeOrder(t *testing.T) {
	ctx := context.Background()
	dir, oids := featureRepo(t)
	r := openRepo(t, dir)
	if err := r.CherryPick(ctx, []string{oids[0] + "^.." + oids[1]}, CherryPickOptions{}); err != nil {
		t.Fatalf("CherryPick range: %v", err)
	}
	if got := fileContent(t, dir, "side.txt"); got != "one\n" {
		t.Fatalf("side.txt = %q, want one", got)
	}
	if got := fileContent(t, dir, "side2.txt"); got != "two\n" {
		t.Fatalf("side2.txt = %q, want two", got)
	}
	count := gitOut(t, dir, "rev-list", "--count", "main")
	if count != "3" {
		t.Fatalf("main has %s commits, want 3", count)
	}
	order := gitOut(t, dir, "log", "--format=%s", "-2")
	if order != "second pick\nfirst pick" {
		t.Fatalf("newest-first log = %q, want second then first", order)
	}
}

func TestCherryPickRecordOriginal(t *testing.T) {
	ctx := context.Background()
	dir, oids := featureRepo(t)
	r := openRepo(t, dir)
	if err := r.CherryPick(ctx, []string{oids[0]}, CherryPickOptions{RecordOriginal: true}); err != nil {
		t.Fatalf("CherryPick -x: %v", err)
	}
	body := gitOut(t, dir, "log", "-1", "--format=%B")
	want := "cherry picked from commit " + oids[0]
	if !strings.Contains(body, want) {
		t.Fatalf("body = %q, want trailer %q", body, want)
	}
}

func TestCherryPickGuards(t *testing.T) {
	ctx := context.Background()
	dir, oids := featureRepo(t)
	r := openRepo(t, dir)
	if err := r.CherryPick(ctx, nil, CherryPickOptions{}); !errors.Is(err, ErrValidationFailed) {
		t.Fatalf("empty refs err = %v, want validation_failed", err)
	}
	if err := r.CherryPick(ctx, []string{"-weird", oids[0]}, CherryPickOptions{}); !errors.Is(err, ErrValidationFailed) {
		t.Fatalf("dash ref err = %v, want validation_failed", err)
	}
	count := gitOut(t, dir, "rev-list", "--count", "main")
	if count != "1" {
		t.Fatalf("guarded calls must not touch history, main has %s commits", count)
	}
}

func TestCherryPickMergeNeedsMainline(t *testing.T) {
	ctx := context.Background()
	dir, mergeHash, _, rootHash := mergeRepo(t)
	gitOut(t, dir, "checkout", "-qb", "consumer", rootHash)
	err := openRepo(t, dir).CherryPick(ctx, []string{mergeHash}, CherryPickOptions{})
	if !errors.Is(err, ErrCommandFailed) {
		t.Fatalf("err = %v, want command_failed refusal without -m", err)
	}
	if !strings.Contains(err.Error(), "merge") {
		t.Fatalf("message = %q, want git's merge-specific refusal", err)
	}
}

func TestCherryPickMergeWithMainline(t *testing.T) {
	ctx := context.Background()
	dir, mergeHash, _, rootHash := mergeRepo(t)
	gitOut(t, dir, "checkout", "-qb", "consumer", rootHash)
	r := openRepo(t, dir)
	if err := r.CherryPick(ctx, []string{mergeHash}, CherryPickOptions{Mainline: 1}); err != nil {
		t.Fatalf("CherryPick -m 1: %v", err)
	}
	if got := fileContent(t, dir, "s.txt"); got != "side content\nline2\n" {
		t.Fatalf("first-parent side content = %q", got)
	}
	if got := fileContent(t, dir, "base.txt"); got != "b\n" {
		t.Fatalf("mainline 1 must not bring first-parent-side edits, base.txt = %q", got)
	}
}

func TestCherryPickNegativeMainline(t *testing.T) {
	ctx := context.Background()
	dir, _ := featureRepo(t)
	err := openRepo(t, dir).CherryPick(ctx, []string{"main"}, CherryPickOptions{Mainline: -1})
	if !errors.Is(err, ErrValidationFailed) {
		t.Fatalf("err = %v, want validation_failed", err)
	}
}

// conflictPickRepo: main commits "main edit" (a.txt), side carries
// "clean pick" (b.txt), "clash pick" (a.txt) and "tail pick" (c.txt).
// Returns the dir and the side oid list in order.
func conflictPickRepo(t *testing.T) (string, []string) {
	t.Helper()
	dir := initRepo(t)
	writeFile(t, dir, "a.txt", "base\n")
	commitAll(t, dir, "base")
	gitOut(t, dir, "checkout", "-qb", "side")
	writeFile(t, dir, "b.txt", "clean\n")
	commitAll(t, dir, "clean pick")
	clean := gitOut(t, dir, "rev-parse", "HEAD")
	writeFile(t, dir, "a.txt", "side version\n")
	commitAll(t, dir, "clash pick")
	clash := gitOut(t, dir, "rev-parse", "HEAD")
	writeFile(t, dir, "c.txt", "tail\n")
	commitAll(t, dir, "tail pick")
	tail := gitOut(t, dir, "rev-parse", "HEAD")
	gitOut(t, dir, "checkout", "-q", "main")
	writeFile(t, dir, "a.txt", "main version\n")
	commitAll(t, dir, "main edit")
	return dir, []string{clean, clash, tail}
}

func startConflictSequence(t *testing.T, r *Repo, oids []string) error {
	t.Helper()
	return r.CherryPick(context.Background(), oids[:2], CherryPickOptions{})
}

func TestCherryPickStateIdle(t *testing.T) {
	dir, _ := conflictPickRepo(t)
	st, err := openRepo(t, dir).CherryPickState(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if st.InProgress || st.Head != "" || len(st.ConflictPaths) != 0 {
		t.Fatalf("state = %+v, want idle", st)
	}
	if err := openRepo(t, dir).CherryPickAbort(context.Background()); !errors.Is(err, ErrCommandFailed) {
		t.Fatalf("abort outside a sequence = %v, want command_failed", err)
	}
}

func TestCherryPickListOrderRespected(t *testing.T) {
	ctx := context.Background()
	dir, oids := featureRepo(t)
	r := openRepo(t, dir)
	if err := r.CherryPick(ctx, []string{oids[1], oids[0]}, CherryPickOptions{}); err != nil {
		t.Fatalf("reverse list pick: %v", err)
	}
	if subjects := gitOut(t, dir, "log", "--format=%s", "-2"); subjects != "first pick\nsecond pick" {
		t.Fatalf("log newest-first = %q, want given order preserved", subjects)
	}
}

func TestCherryPickConflictTypedWithState(t *testing.T) {
	ctx := context.Background()
	dir, oids := conflictPickRepo(t)
	r := openRepo(t, dir)
	err := startConflictSequence(t, r, oids)
	if !errors.Is(err, ErrCherryPickConflict) {
		t.Fatalf("err = %v, want cherry_pick_conflict", err)
	}
	st, err := r.CherryPickState(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !st.InProgress {
		t.Fatalf("state = %+v, want in progress", st)
	}
	if st.Head != oids[1] {
		t.Fatalf("Head = %q, want clashing commit %q", st.Head, oids[1])
	}
	if len(st.ConflictPaths) != 1 || st.ConflictPaths[0] != "a.txt" {
		t.Fatalf("ConflictPaths = %v, want [a.txt]", st.ConflictPaths)
	}
	// first pick landed before the clash
	if got := fileContent(t, dir, "b.txt"); got != "clean\n" {
		t.Fatalf("earlier pick lost: b.txt = %q", got)
	}
}

func TestCherryPickAbortRollsBackSequence(t *testing.T) {
	ctx := context.Background()
	dir, oids := conflictPickRepo(t)
	r := openRepo(t, dir)
	before := gitOut(t, dir, "rev-parse", "HEAD")
	if err := startConflictSequence(t, r, oids); !errors.Is(err, ErrCherryPickConflict) {
		t.Fatal(err)
	}
	if err := r.CherryPickAbort(ctx); err != nil {
		t.Fatalf("abort: %v", err)
	}
	st, err := r.CherryPickState(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if st.InProgress {
		t.Fatalf("state = %+v, want idle after abort", st)
	}
	if got := gitOut(t, dir, "rev-parse", "HEAD"); got != before {
		t.Fatalf("abort must return to %q, got %q", before, got)
	}
	if got := fileContent(t, dir, "b.txt"); got != "" {
		t.Fatalf("abort must undo the landed pick, b.txt = %q", got)
	}
}

func TestCherryPickResolveAndContinue(t *testing.T) {
	ctx := context.Background()
	dir, oids := conflictPickRepo(t)
	r := openRepo(t, dir)
	if err := startConflictSequence(t, r, oids); !errors.Is(err, ErrCherryPickConflict) {
		t.Fatal(err)
	}
	writeFile(t, dir, "a.txt", "merged by hand\n")
	gitOut(t, dir, "add", "a.txt")
	if err := r.CherryPickContinue(ctx); err != nil {
		t.Fatalf("continue: %v", err)
	}
	st, err := r.CherryPickState(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if st.InProgress {
		t.Fatalf("state = %+v, want done after continue", st)
	}
	if got := fileContent(t, dir, "a.txt"); got != "merged by hand\n" {
		t.Fatalf("resolution lost: a.txt = %q", got)
	}
	subjects := gitOut(t, dir, "log", "--format=%s", "-3")
	if subjects != "clash pick\nclean pick\nmain edit" {
		t.Fatalf("log after continue = %q", subjects)
	}
	if got := gitOut(t, dir, "rev-list", "--count", "HEAD"); got != "4" {
		t.Fatalf("continue must finish both picks, count = %s, want 4", got)
	}
}

func TestCherryPickContinueWithoutResolutionFails(t *testing.T) {
	ctx := context.Background()
	dir, oids := conflictPickRepo(t)
	r := openRepo(t, dir)
	if err := startConflictSequence(t, r, oids); !errors.Is(err, ErrCherryPickConflict) {
		t.Fatal(err)
	}
	err := r.CherryPickContinue(ctx)
	if !errors.Is(err, ErrCommandFailed) {
		t.Fatalf("err = %v, want git's unmerged-files refusal", err)
	}
	st, stateErr := r.CherryPickState(ctx)
	if stateErr != nil {
		t.Fatal(stateErr)
	}
	if !st.InProgress || len(st.ConflictPaths) != 1 {
		t.Fatalf("state = %+v, want conflict still pending", st)
	}
}

func TestCherryPickSkipGoesToNext(t *testing.T) {
	ctx := context.Background()
	dir, oids := conflictPickRepo(t)
	r := openRepo(t, dir)
	err := r.CherryPick(ctx, oids, CherryPickOptions{})
	if !errors.Is(err, ErrCherryPickConflict) {
		t.Fatalf("err = %v, want cherry_pick_conflict on the clash", err)
	}
	// remaining sequence holds only the tail pick; skip drops the clash
	if err := r.CherryPickSkip(ctx); err != nil {
		t.Fatalf("skip: %v", err)
	}
	st, err := r.CherryPickState(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if st.InProgress {
		t.Fatalf("state = %+v, want idle after finishing tail pick", st)
	}
	if got := fileContent(t, dir, "c.txt"); got != "tail\n" {
		t.Fatalf("tail pick missing: c.txt = %q", got)
	}
	if got := fileContent(t, dir, "a.txt"); got != "main version\n" {
		t.Fatalf("skipped clash must not touch a.txt, got %q", got)
	}
}

func TestCherryPickAlreadyAppliedIsTypedEmpty(t *testing.T) {
	ctx := context.Background()
	dir, oids := featureRepo(t)
	r := openRepo(t, dir)
	if err := r.CherryPick(ctx, []string{oids[0]}, CherryPickOptions{}); err != nil {
		t.Fatal(err)
	}
	err := r.CherryPick(ctx, []string{oids[0]}, CherryPickOptions{})
	if !errors.Is(err, ErrCherryPickEmpty) {
		t.Fatalf("err = %v, want cherry_pick_empty for already-applied pick", err)
	}
	st, err := r.CherryPickState(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !st.InProgress || len(st.ConflictPaths) != 0 {
		t.Fatalf("state = %+v, want stopped without conflicts", st)
	}
	if err := r.CherryPickAbort(ctx); err != nil {
		t.Fatalf("abort after empty: %v", err)
	}
	if got := gitOut(t, dir, "rev-list", "--count", "main"); got != "2" {
		t.Fatalf("count = %s, want only the first pick", got)
	}
}

func TestCherryPickStateUnreadableHead(t *testing.T) {
	skipWithoutUnixPerms(t)
	r := openRepo(t, featureRepoOnly(t))
	head := filepath.Join(r.path, ".git", "HEAD")
	if err := os.Chmod(head, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(head, 0o644) })
	_, err := r.CherryPickState(context.Background())
	ge, ok := err.(*GitError)
	if !ok {
		t.Fatalf("err = %v, want GitError", err)
	}
	if ge.ExitCode == 1 {
		t.Fatalf("unreadable HEAD mapped to idle: %+v", ge)
	}
}

func TestCherryPickStateStatusFailurePropagates(t *testing.T) {
	skipWithoutUnixPerms(t)
	ctx := context.Background()
	dir, oids := conflictPickRepo(t)
	r := openRepo(t, dir)
	if err := r.CherryPick(ctx, oids[:2], CherryPickOptions{}); !errors.Is(err, ErrCherryPickConflict) {
		t.Fatal(err)
	}
	index := filepath.Join(dir, ".git", "index")
	if err := os.Chmod(index, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(index, 0o644) })
	if _, err := r.CherryPickState(ctx); !errors.Is(err, ErrCommandFailed) {
		t.Fatalf("err = %v, want status failure propagated", err)
	}
}

func featureRepoOnly(t *testing.T) string {
	t.Helper()
	dir, _ := featureRepo(t)
	return dir
}
