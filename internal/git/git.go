package git

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"time"
)

// buildGitCmd is the single place git binaries are constructed: machine
// flags first, work dir pinned, non-interactive env defaults on top of the
// user's environment so credential helpers and aliases keep working.
// args are passed verbatim as argv - no shell is involved.
func buildGitCmd(ctx context.Context, repoPath string, args ...string) *exec.Cmd {
	machineArgs := append([]string{"--no-pager", "-c", "color.ui=never"}, args...)
	cmd := exec.CommandContext(ctx, "git", machineArgs...)
	// ctx done kills git; WaitDelay then caps how long we keep waiting for
	// output pipes that killed git's children (hooks, credential helpers)
	// may still hold open.
	cmd.WaitDelay = 2 * time.Second
	cmd.Dir = repoPath
	cmd.Env = append(os.Environ(),
		"GIT_TERMINAL_PROMPT=0",
		"LC_ALL=C",
		"GIT_PAGER=cat",
	)
	return cmd
}

// runGit executes git in repoPath and returns its streams. Any non-zero exit
// (and any ctx kill) surfaces as a *GitError.
func runGit(ctx context.Context, repoPath string, args ...string) (stdout, stderr []byte, err error) {
	cmd := buildGitCmd(ctx, repoPath, args...)
	var out, errOut bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errOut

	if runErr := cmd.Run(); runErr != nil {
		return out.Bytes(), errOut.Bytes(), classify(ctx, errOut.String(), exitCodeOf(runErr))
	}
	return out.Bytes(), errOut.Bytes(), nil
}

// exitCodeOf digs the child status out of exec errors; signals and spawn
// failures report -1.
func exitCodeOf(runErr error) int {
	var exitErr *exec.ExitError
	if errors.As(runErr, &exitErr) {
		return exitErr.ExitCode()
	}
	return -1
}
