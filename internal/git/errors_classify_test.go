package git

import (
	"context"
	"errors"
	"testing"
)

func TestRunGitPullWithoutUpstreamMapsNoUpstream(t *testing.T) {
	dir := initRepo(t)
	commitRepo(t, dir, "first")

	_, _, err := runGit(context.Background(), dir, "pull")
	if !errors.Is(err, ErrNoUpstream) {
		t.Fatalf("err = %v, want ErrNoUpstream", err)
	}
}

func TestRunGitMergeConflictMapsConflict(t *testing.T) {
	dir := conflictingRepo(t)

	_, stderr, err := runGit(context.Background(), dir, "merge", "feature")
	if !errors.Is(err, ErrConflict) {
		t.Fatalf("err = %v, want ErrConflict; stderr=%q", err, stderr)
	}
}

func TestRunGitStreamMergeConflictMapsConflict(t *testing.T) {
	dir := conflictingRepo(t)

	err := runGitStream(context.Background(), dir, func(string) {}, "merge", "feature")
	if !errors.Is(err, ErrConflict) {
		t.Fatalf("err = %v, want ErrConflict", err)
	}
}

func TestClassifyAuthSignatures(t *testing.T) {
	ctx := context.Background()
	cases := []string{
		"fatal: could not read Username for 'https://github.com': terminal prompts disabled",
		"remote: Error: Authentication failed",
	}
	for _, stderr := range cases {
		if err := classify(ctx, stderr, 128); !errors.Is(err, ErrAuthFailed) {
			t.Fatalf("classify(%q) = %v, want ErrAuthFailed", stderr, err)
		}
	}
}
