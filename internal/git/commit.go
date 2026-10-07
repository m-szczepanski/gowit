package git

import (
	"context"
	"errors"
	"strings"
)

// CommitOptions describes `git commit` invocations. Message is passed as a
// single argv value (never through a shell), so any quoting or injection in
// user text is inert. OnOutput, when set, receives every stdout/stderr line
// live - hook chatter included - while Commit keeps its own tail for error
// reporting.
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

	tail := newOutputTail(maxCommitTailLines)
	onLine := func(line string) {
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
		t.lines = t.lines[len(t.lines)-t.max:]
	}
}

func (t *outputTail) text() string {
	return strings.Join(t.lines, "\n")
}

// classifyCommitError maps git commit's textual failure modes to typed
// errors. Git itself stays silent when a hook rejects a commit - it just
// forwards the hook's output and exits nonzero - so rejection is detected
// by elimination: the failure is not git's own "nothing to commit" family
// and no output line is a git-authored error (fatal:/error: with
// LC_ALL=C). That residual class can only be something running inside the
// commit pipeline (a hook, or filter/editor tooling) aborting before git
// spoke; its output is exactly what the user needs to see. The tail is
// attached to every branch because the first line of a failed commit is
// rarely the informative one.
func classifyCommitError(err error, tail *outputTail) error {
	var ge *GitError
	if !errors.As(err, &ge) {
		return err
	}
	if ge.Code == CodeTimeout {
		return ge
	}
	text := tail.text()
	switch {
	case strings.Contains(text, "nothing to commit"),
		strings.Contains(text, "no changes added to commit"):
		return &GitError{Code: CodeNothingToCommit, Message: actionableLine(text), ExitCode: ge.ExitCode}
	case hasGitAuthoredError(text):
		ge.Message = text
		return ge
	default:
		message := text
		if message == "" {
			message = "commit aborted without output (a hook exited nonzero)"
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

// actionableLine returns git's own summary line ("nothing to commit..." /
// "no changes added to commit..."), skipping the status listing that
// precedes it.
func actionableLine(text string) string {
	lines := strings.Split(text, "\n")
	for _, line := range lines {
		if strings.Contains(line, "nothing to commit") || strings.Contains(line, "no changes added to commit") {
			return line
		}
	}
	for _, line := range lines {
		if line != "" && !strings.HasPrefix(line, "On branch") {
			return line
		}
	}
	return text
}
