package git

import (
	"context"
	"errors"
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
