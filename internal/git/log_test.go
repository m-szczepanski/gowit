package git

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLogLinearHistory(t *testing.T) {
	ctx := context.Background()
	dir := initRepo(t)
	writeFile(t, dir, "f.txt", "x\n")
	commitAll(t, dir, "first\n\nbody A\nbody B")
	writeFile(t, dir, "f.txt", "y\n")
	commitAll(t, dir, "ünïcode ✨ subject")
	writeFile(t, dir, "f.txt", "z\n")
	commitAll(t, dir, "last")
	runGit(ctx, dir, "tag", "v0.1")

	commits, err := openRepo(t, dir).Log(ctx, LogOptions{})
	if err != nil {
		t.Fatalf("Log: %v", err)
	}
	if len(commits) != 3 {
		t.Fatalf("got %d commits, want 3", len(commits))
	}
	first, middle, latest := commits[2], commits[1], commits[0]

	if first.Hash != hashOf(t, dir, "HEAD~2") || first.Subject != "first" || first.Body != "body A\nbody B" {
		t.Fatalf("first = %+v", first)
	}
	if len(first.ParentHashes) != 0 {
		t.Fatalf("root parents = %v, want none", first.ParentHashes)
	}
	if !strings.HasPrefix(first.Hash, first.ShortHash) || len(first.ShortHash) < 7 || first.ShortHash != first.Hash[:len(first.ShortHash)] {
		t.Fatalf("short hash %q inconsistent with %q", first.ShortHash, first.Hash)
	}
	if middle.Subject != "ünïcode ✨ subject" || middle.Body != "" {
		t.Fatalf("middle = %+v, want unicode subject, empty body", middle)
	}
	if latest.Hash != hashOf(t, dir, "HEAD") || len(latest.ParentHashes) != 1 || latest.ParentHashes[0] != middle.Hash {
		t.Fatalf("latest = %+v, want one parent = middle", latest)
	}
	if got := latest.Refs; len(got) != 2 || got[0] != (Ref{Kind: RefHead, Name: "main"}) || got[1] != (Ref{Kind: RefTag, Name: "v0.1"}) {
		t.Fatalf("latest refs = %+v, want HEAD->main and tag v0.1", got)
	}
	if latest.AuthorName != "test" || latest.AuthorEmail != "t@t" {
		t.Fatalf("identity = %q %q", latest.AuthorName, latest.AuthorEmail)
	}
	if latest.AuthorDate.IsZero() || !latest.AuthorDate.Equal(latest.CommitterDate) {
		t.Fatalf("dates = %v %v, want equal non-zero", latest.AuthorDate, latest.CommitterDate)
	}
	if d := time.Since(latest.AuthorDate); d < -time.Minute || d > time.Minute {
		t.Fatalf("author date %v not near now", latest.AuthorDate)
	}
}

func TestLogUnbornRepoIsEmpty(t *testing.T) {
	dir := initRepo(t)
	commits, err := openRepo(t, dir).Log(context.Background(), LogOptions{})
	if err != nil {
		t.Fatalf("Log: %v", err)
	}
	if len(commits) != 0 {
		t.Fatalf("got %+v, want empty", commits)
	}
}

func TestLogCtxKill(t *testing.T) {
	dir := initRepo(t)
	writeFile(t, dir, "f.txt", "x\n")
	commitAll(t, dir, "one")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := openRepo(t, dir).Log(ctx, LogOptions{}); !errors.Is(err, ErrTimeout) {
		t.Fatalf("err = %v, want timeout", err)
	}
}

func TestLogPropagatesGitFailure(t *testing.T) {
	ctx := context.Background()
	dir := initRepo(t)
	writeFile(t, dir, "f.txt", "x\n")
	commitAll(t, dir, "root")
	writeFile(t, dir, "f.txt", "y\n")
	commitAll(t, dir, "tip")

	// HEAD itself resolves, so the unborn guard passes, but walking parents
	// reads the corrupted root commit object and git dies mid-log
	hash := hashOf(t, dir, "HEAD~1")
	object := filepath.Join(dir, ".git", "objects", hash[:2], hash[2:])
	// loose objects are committed read-only
	if err := os.Chmod(object, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(object, []byte("garbage-not-a-commit"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := openRepo(t, dir).Log(ctx, LogOptions{}); !errors.Is(err, ErrCommandFailed) {
		t.Fatalf("err = %v, want command_failed", err)
	}
}

func TestLogPagination(t *testing.T) {
	ctx := context.Background()
	dir := initRepo(t)
	writeFile(t, dir, "f.txt", "0\n")
	commitAll(t, dir, "c0")
	for i := 1; i < 5; i++ {
		writeFile(t, dir, "f.txt", fmt.Sprint(i)+"\n")
		commitAll(t, dir, fmt.Sprint("c", i))
	}
	repo := openRepo(t, dir)
	all, err := repo.Log(ctx, LogOptions{})
	if err != nil || len(all) != 5 {
		t.Fatalf("all = %d commits err %v, want 5", len(all), err)
	}

	page, err := repo.Log(ctx, LogOptions{MaxCount: 2})
	if err != nil {
		t.Fatal(err)
	}
	if len(page) != 2 || page[0].Subject != "c4" || page[1].Subject != "c3" {
		t.Fatalf("first page = %+v, want c4, c3", subjects(page))
	}

	page, err = repo.Log(ctx, LogOptions{MaxCount: 2, Skip: 2})
	if err != nil {
		t.Fatal(err)
	}
	if len(page) != 2 || page[0].Subject != "c2" || page[1].Subject != "c1" {
		t.Fatalf("second page = %+v, want c2, c1", subjects(page))
	}

	// window reaching past the oldest commit clamps instead of failing
	page, err = repo.Log(ctx, LogOptions{MaxCount: 2, Skip: 4})
	if err != nil {
		t.Fatal(err)
	}
	if len(page) != 1 || page[0].Subject != "c0" {
		t.Fatalf("tail page = %+v, want just c0", subjects(page))
	}

	page, err = repo.Log(ctx, LogOptions{Skip: 50})
	if err != nil {
		t.Fatal(err)
	}
	if len(page) != 0 {
		t.Fatalf("past-end page = %+v, want empty", subjects(page))
	}

	if _, err := repo.Log(ctx, LogOptions{MaxCount: -1}); !errors.Is(err, ErrValidationFailed) {
		t.Fatalf("err = %v, want validation_failed", err)
	}
	if _, err := repo.Log(ctx, LogOptions{Skip: -2}); !errors.Is(err, ErrValidationFailed) {
		t.Fatalf("err = %v, want validation_failed", err)
	}
}

func subjects(cs []Commit) []string {
	out := []string{}
	for _, c := range cs {
		out = append(out, c.Subject)
	}
	return out
}
