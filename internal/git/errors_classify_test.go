package git

import (
	"context"
	"errors"
	"strings"
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
		"git@github.com: Permission denied (publickey).",
	}
	for _, stderr := range cases {
		if err := classify(ctx, stderr, 128); !errors.Is(err, ErrAuthFailed) {
			t.Fatalf("classify(%q) = %v, want ErrAuthFailed", stderr, err)
		}
	}
}

// WI3 of #34: an auth failure must point users at the system setup
// instead of only echoing git's raw line.
func TestClassifyAuthFailureCarriesGuidance(t *testing.T) {
	stderr := "git@github.com: Permission denied (publickey)."
	ge := classify(context.Background(), stderr, 128)
	if ge.Code != CodeAuthFailed {
		t.Fatalf("classify = %+v, want auth_failed", ge)
	}
	if !strings.HasPrefix(ge.Message, stderr) {
		t.Fatalf("message = %q, want git's line preserved first", ge.Message)
	}
	if !strings.Contains(ge.Message, "SSH agent") || !strings.Contains(ge.Message, "credential helper") {
		t.Fatalf("message = %q, want guidance naming the SSH agent and credential helper", ge.Message)
	}
}

func TestClassifyNonAuthKeepsPlainMessage(t *testing.T) {
	ge := classify(context.Background(), "fatal: 'x' does not appear to be a git repository", 128)
	if strings.Contains(ge.Message, "SSH agent") {
		t.Fatalf("guidance leaked into a non-auth message: %q", ge.Message)
	}
}
