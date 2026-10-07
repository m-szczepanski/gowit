package git

import (
	"context"
	"errors"
	"slices"
	"strings"
	"sync"
)

// CommitOptions describes `git commit` invocations. Message is passed as a
// single argv value (never through a shell), so any quoting or injection in
// user text is inert. OnOutput, when set, receives every stdout/stderr
// line live - hook chatter included - called sequentially under an internal
// lock, so it need not be goroutine-safe but must not block long.
type CommitOptions struct {
	Message    string
	Amend      bool
	AllowEmpty bool
	OnOutput   func(line string)
}

// Commit commits the staged index. Amending with an empty message keeps the
// previous message (--no-edit); a fresh commit demands a message and fails
// validation before git is invoked. Hook failures, "nothing to commit" and
// "no staged changes" surface as typed GitErrors carrying the output tail.
func (r *Repo) Commit(ctx context.Context, opts CommitOptions) error {
	message := strings.TrimSpace(opts.Message)
	if message == "" && !opts.Amend {
		return &GitError{Code: CodeValidationFailed, Message: "commit message required", ExitCode: -1}
	}

	args := []string{"commit"}
	if opts.Amend {
		args = append(args, "--amend")
		if message == "" {
			args = append(args, "--no-edit")
		}
	}
	if message != "" {
		args = append(args, "-m", message)
	}
	if opts.AllowEmpty {
		args = append(args, "--allow-empty")
	}

	// runGitStream feeds stdout and stderr from separate goroutines; the
	// lock keeps the tail consistent and guarantees sequential OnOutput
	// calls, so downstream consumers need not be goroutine-safe.
	var mu sync.Mutex
	tail := newOutputTail(maxCommitTailLines)
	onLine := func(line string) {
		mu.Lock()
		defer mu.Unlock()
		tail.add(line)
		if opts.OnOutput != nil {
			opts.OnOutput(line)
		}
	}
	err := runGitStream(ctx, r.path, onLine, args...)
	if err == nil {
		return nil
	}
	return classifyCommitError(err, tail)
}

const maxCommitTailLines = 24

// outputTail keeps the last n lines of git output for error reporting:
// hook rejections print their reason early and git's own line last, so a
// bounded tail preserves both without retaining multi-MB installs.
type outputTail struct {
	lines []string
	max   int
}

func newOutputTail(max int) *outputTail {
	return &outputTail{max: max}
}

func (t *outputTail) add(line string) {
	t.lines = append(t.lines, line)
	if len(t.lines) > t.max {
		// clone so trimmed strings stop pinning the shared backing array
		t.lines = slices.Clone(t.lines[len(t.lines)-t.max:])
	}
}

func (t *outputTail) text() string {
	return strings.Join(t.lines, "\n")
}

// classifyCommitError maps git commit's textual failure modes to typed
// errors. Git prints nothing of its own when a hook rejects a commit - it
// forwards the hook's output and exits 1 - so rejection is detected by
// elimination: the failure is neither git's "nothing to commit" family nor
// a git-authored error line (fatal:/error:, stable under LC_ALL=C); the
// residual class is something in the commit pipeline (hook, or
// filter/editor tooling) aborting before git spoke. Two known limits,
// accepted because the output tail always survives: a hook echoing "error:
// ..." is classified command_failed, and a hook whose final line happens to
// quote git's own summary is classified nothing_to_commit.
func classifyCommitError(err error, tail *outputTail) error {
	var ge *GitError
	if !errors.As(err, &ge) {
		return err
	}
	if ge.Code == CodeTimeout || ge.ExitCode == -1 {
		// ctx kill or spawn failure: git never ran to completion, so
		// nothing in the residual class reasoning applies
		return ge
	}
	text := tail.text()
	last := lastNonEmptyLine(text)
	switch {
	case strings.Contains(last, "nothing to commit"),
		strings.Contains(last, "no changes added to commit"):
		return &GitError{Code: CodeNothingToCommit, Message: last, ExitCode: ge.ExitCode}
	case hasGitAuthoredError(text):
		ge.Message = text
		return ge
	default:
		message := text
		if message == "" {
			message = "commit aborted without output (an external command in the commit pipeline exited nonzero)"
		}
		return &GitError{Code: CodeCommitRejected, Message: message, ExitCode: ge.ExitCode}
	}
}

// hasGitAuthoredError reports a line git itself printed as an error.
func hasGitAuthoredError(text string) bool {
	for _, line := range strings.Split(text, "\n") {
		if strings.HasPrefix(line, "fatal:") || strings.HasPrefix(line, "error:") {
			return true
		}
	}
	return false
}

// lastNonEmptyLine returns the final non-blank line, where git's own
// summary always lands on a failed commit.
func lastNonEmptyLine(text string) string {
	lines := strings.Split(text, "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		if lines[i] != "" {
			return lines[i]
		}
	}
	return ""
}
