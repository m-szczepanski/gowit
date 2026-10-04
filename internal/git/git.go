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
	var stderr bytes.Buffer
	out := &lineSplitter{onLine: onLine}
	errOut := &lineSplitter{onLine: onLine, captured: &stderr}
	cmd.Stdout = out
	cmd.Stderr = errOut

	runErr := cmd.Run()
	out.flush()
	errOut.flush()
	if runErr != nil {
		return classify(ctx, stderr.String(), exitCodeOf(runErr))
	}
	return nil
}
