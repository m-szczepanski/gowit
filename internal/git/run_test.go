package git

import (
	"context"
	"errors"
	"path/filepath"
	"slices"
	"strings"
	"testing"
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

func TestBuildGitCmdSetsWorkdirMachineFlagsAndEnv(t *testing.T) {
	cmd := buildGitCmd(context.Background(), filepath.Join("some", "dir"), "status", "--porcelain=v2")

	if cmd.Dir != filepath.Join("some", "dir") {
		t.Fatalf("cmd.Dir = %q", cmd.Dir)
	}
	wantArgs := []string{"--no-pager", "-c", "color.ui=never", "status", "--porcelain=v2"}
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
	// user configuration must still be honored (credential helpers, aliases)
	if !slices.ContainsFunc(env, func(kv string) bool { return strings.HasPrefix(kv, "HOME=") }) {
		t.Fatal("os.Environ base not preserved")
	}
}
