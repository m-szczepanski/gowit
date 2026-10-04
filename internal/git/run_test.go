package git

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

func TestRunGitReturnsStdoutStderr(t *testing.T) {
	dir := initRepo(t)

	stdout, stderr, err := runGit(context.Background(), dir, "rev-parse", "--is-inside-work-tree")
	if err != nil {
		t.Fatalf("runGit: %v", err)
	}
	if string(stdout) != "true\n" {
		t.Fatalf("stdout = %q, want %q", stdout, "true\n")
	}
	if len(stderr) != 0 {
		t.Fatalf("stderr = %q, want empty", stderr)
	}
}

func TestRunGitNonRepoMapsTypedNotARepository(t *testing.T) {
	// Real git exits 128 with "fatal: not a git repository" outside a work tree.
	_, _, err := runGit(context.Background(), t.TempDir(), "rev-parse", "--is-inside-work-tree")

	var gitErr *GitError
	if !errors.As(err, &gitErr) {
		t.Fatalf("err = %v, want *GitError", err)
	}
	if !errors.Is(err, ErrNotARepository) {
		t.Fatalf("err = %v, want ErrNotARepository", err)
	}
	if gitErr.ExitCode != 128 {
		t.Fatalf("ExitCode = %d, want 128 (git fatal convention)", gitErr.ExitCode)
	}
}

func TestRunGitUnknownFailureMapsCommandFailed(t *testing.T) {
	dir := initRepo(t)

	_, _, err := runGit(context.Background(), dir, "checkout", "no-such-branch")
	var gitErr *GitError
	if !errors.As(err, &gitErr) {
		t.Fatalf("err = %v, want *GitError", err)
	}
	if !errors.Is(err, ErrCommandFailed) {
		t.Fatalf("err = %v, want ErrCommandFailed", err)
	}
}

func TestRunGitSpawnFailureMapsCommandFailed(t *testing.T) {
	t.Setenv("PATH", "")

	_, _, err := runGit(context.Background(), t.TempDir(), "version")
	var gitErr *GitError
	if !errors.As(err, &gitErr) {
		t.Fatalf("err = %v, want *GitError", err)
	}
	if !errors.Is(err, ErrCommandFailed) {
		t.Fatalf("err = %v, want ErrCommandFailed", err)
	}
	if gitErr.ExitCode != -1 {
		t.Fatalf("ExitCode = %d, want -1 for spawn failure", gitErr.ExitCode)
	}
}

func TestRunGitKillsOnContextTimeout(t *testing.T) {
	dir := initRepo(t)
	hook := filepath.Join(dir, ".git", "hooks", "pre-commit")
	// a blocking hook keeps git alive past the deadline on every OS:
	// Git for Windows runs hooks through its bundled sh, where sleep exists.
	// sleep must finish before the test ends or its sh keeps the temp dir
	// locked and t.TempDir cleanup fails on Windows
	if err := os.WriteFile(hook, []byte("#!/bin/sh\nsleep 2\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	started := time.Now()
	_, _, err := runGit(ctx, dir, "commit", "--allow-empty", "-m", "slow")
	elapsed := time.Since(started)

	var gitErr *GitError
	if !errors.As(err, &gitErr) {
		t.Fatalf("err = %v, want *GitError", err)
	}
	if !errors.Is(err, ErrTimeout) {
		t.Fatalf("err = %v, want ErrTimeout", err)
	}
	// Kill alone is not enough: the hook's `sleep` grandchild holds the output
	// pipes open, so runGit must also stop waiting on the streams (WaitDelay).
	if elapsed > 10*time.Second {
		t.Fatalf("runGit took %v after timeout, want < 10s", elapsed)
	}
	if !strings.Contains(gitErr.Message, "killed") {
		t.Fatalf("Message = %q, want mention of kill", gitErr.Message)
	}
}

func TestRunGitSurvivesPipeHoldingGrandchild(t *testing.T) {
	dir := initRepo(t)
	commitRepo(t, dir, "base")
	// detached post-commit holder keeps stdout's write-end open past the
	// 2s WaitDelay after git itself exited 0: Run returns ErrWaitDelay,
	// the operation must still count as success
	hook := filepath.Join(dir, ".git", "hooks", "post-commit")
	if err := os.WriteFile(hook, []byte("#!/bin/sh\nsleep 3 2>/dev/null &\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	out, stderr, err := runGit(context.Background(), dir, "commit", "--allow-empty", "-m", "held")
	if err != nil {
		t.Fatalf("successful commit with pipe-holding grandchild failed: %v (stderr %q)", err, stderr)
	}
	if !strings.Contains(string(out), "held") {
		t.Fatalf("stdout = %q, want commit summary", out)
	}
	// let the detached holder exit before t.TempDir cleanup; on Windows a
	// live grandchild keeps the repo directory locked
	time.Sleep(1 * time.Second)
}

func TestBuildGitCmdSetsWorkdirMachineFlagsAndEnv(t *testing.T) {
	cmd := buildGitCmd(context.Background(), filepath.Join("some", "dir"), "status", "--porcelain=v2")

	if cmd.Dir != filepath.Join("some", "dir") {
		t.Fatalf("cmd.Dir = %q", cmd.Dir)
	}
	wantArgs := []string{"--no-pager",
		"-c", "color.ui=never",
		"-c", "color.diff=never",
		"-c", "color.status=never",
		"-c", "color.branch=never",
		"status", "--porcelain=v2"}
	if len(cmd.Args) != len(wantArgs)+1 { // argv[0] is the binary path
		t.Fatalf("cmd.Args = %q, want %q prefixed by git", cmd.Args, wantArgs)
	}
	for i, want := range wantArgs {
		if cmd.Args[i+1] != want {
			t.Fatalf("cmd.Args[%d] = %q, want %q", i+1, cmd.Args[i+1], want)
		}
	}

	env := cmd.Env
	for _, want := range []string{"GIT_TERMINAL_PROMPT=0", "LC_ALL=C", "GIT_PAGER=cat"} {
		if !slices.Contains(env, want) {
			t.Fatalf("env missing %s: %v", want, env)
		}
	}
	// user configuration must still be honored (credential helpers, aliases):
	// the full parent environment is carried, machine vars only appended
	if len(cmd.Env) != len(os.Environ())+3 {
		t.Fatalf("env = %d entries, want os.Environ (%d) + 3 defaults", len(cmd.Env), len(os.Environ()))
	}
}
