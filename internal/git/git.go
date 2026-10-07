package git

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"strings"
	"time"
)

// buildGitCmd is the single place git binaries are constructed: machine
// flags first, work dir pinned, non-interactive env defaults on top of the
// user's environment so credential helpers and aliases keep working.
// args are passed verbatim as argv - no shell is involved.
// The per-slot color overrides are load-bearing: a user-level
// color.diff=always beats color.ui=never (git config precedence) and would
// inject ANSI into parsed output. The diff prefix overrides keep "a/" and
// "b/" in file headers regardless of user config, which parseUnifiedDiff
// relies on.
func buildGitCmd(ctx context.Context, repoPath string, args ...string) *exec.Cmd {
	machineArgs := append([]string{
		"--no-pager",
		"-c", "color.ui=never",
		"-c", "color.diff=never",
		"-c", "color.status=never",
		"-c", "color.branch=never",
		"-c", "diff.noprefix=false",
		"-c", "diff.mnemonicPrefix=false",
	}, args...)
	cmd := exec.CommandContext(ctx, "git", machineArgs...)
	// ctx done kills git; WaitDelay then caps how long we keep waiting for
	// output pipes that killed git's children (hooks, credential helpers)
	// may still hold open. A git that already exited 0 with a pipe-holding
	// grandchild still counts as success via toRunError.
	cmd.WaitDelay = 2 * time.Second
	cmd.Dir = repoPath
	cmd.Env = append(os.Environ(),
		"GIT_TERMINAL_PROMPT=0",
		"LC_ALL=C",
		"GIT_PAGER=cat",
	)
	return cmd
}

// toRunError resolves cmd.Run's error into the typed model. ErrWaitDelay
// after an exited-0 git is a stream-closure timeout, not a failed operation.
func toRunError(ctx context.Context, cmd *exec.Cmd, runErr error, output string) error {
	if runErr == nil {
		return nil
	}
	if errors.Is(runErr, exec.ErrWaitDelay) && cmd.ProcessState != nil && cmd.ProcessState.Success() {
		return nil
	}
	return classify(ctx, output, exitCodeOf(runErr))
}

// runGit executes git in repoPath and returns its streams. Any non-zero exit
// (and any ctx kill) surfaces as a *GitError. Callers passing user-controlled
// paths must add "--" themselves (runGit cannot know which args are paths).
func runGit(ctx context.Context, repoPath string, args ...string) (stdout, stderr []byte, err error) {
	cmd := buildGitCmd(ctx, repoPath, args...)
	var out, errOut bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errOut

	runErr := cmd.Run()
	// status lines (merge CONFLICT) land on stdout, errors on stderr;
	// classify against both so signatures are caught wherever they appear
	return out.Bytes(), errOut.Bytes(), toRunError(ctx, cmd, runErr, errOut.String()+"\n"+out.String())
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

// lineSplitter accumulates writes and forwards completed lines; stdout and
// stderr each get one instance, so onLine may be called concurrently per
// stream but never concurrently for the same stream.
type lineSplitter struct {
	onLine   func(line string)
	partial  strings.Builder
	captured *bytes.Buffer // optional full copy, for stderr classification
}

func (w *lineSplitter) Write(p []byte) (int, error) {
	if w.captured != nil {
		w.captured.Write(p)
	}
	text := w.partial.String() + string(p)
	w.partial.Reset()
	for len(text) > 0 {
		i := strings.IndexByte(text, '\n')
		if i < 0 {
			w.partial.WriteString(text)
			break
		}
		w.onLine(strings.TrimRight(text[:i], "\r"))
		text = text[i+1:]
	}
	return len(p), nil
}

func (w *lineSplitter) flush() {
	if rest := w.partial.String(); rest != "" {
		w.onLine(rest)
		w.partial.Reset()
	}
}

// runGitStream executes git and feeds each output line (stdout and stderr;
// fetch/push progress lands on stderr) to onLine while the process runs.
// Use it for long ops that must surface progress; runGit for reads.
func runGitStream(ctx context.Context, repoPath string, onLine func(line string), args ...string) error {
	cmd := buildGitCmd(ctx, repoPath, args...)
	var outBuf, errBuf bytes.Buffer
	out := &lineSplitter{onLine: onLine, captured: &outBuf}
	errOut := &lineSplitter{onLine: onLine, captured: &errBuf}
	cmd.Stdout = out
	cmd.Stderr = errOut

	runErr := cmd.Run()
	out.flush()
	errOut.flush()
	return toRunError(ctx, cmd, runErr, errBuf.String()+"\n"+outBuf.String())
}
