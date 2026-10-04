package git

import (
	"context"
	"fmt"
	"strings"
)

// ErrorCode names a class of git failure. Codes are stable identifiers for
// the UI: switch on them, not on error text.
type ErrorCode string

const (
	CodeNotARepository ErrorCode = "not_a_repository"
	CodeNoUpstream     ErrorCode = "no_upstream"
	CodeConflict       ErrorCode = "conflict"
	CodeAuthFailed     ErrorCode = "auth_failed"
	CodeTimeout        ErrorCode = "timeout"
	CodeCommandFailed  ErrorCode = "command_failed"
)

// GitError is the only error type that crosses to the app layer (ARCHITECTURE.md §4).
type GitError struct {
	Code     ErrorCode
	Message  string
	ExitCode int
}

func (e *GitError) Error() string {
	return fmt.Sprintf("git %s: %s (exit %d)", e.Code, e.Message, e.ExitCode)
}

// Is matches on Code so callers use errors.Is against the sentinels below.
func (e *GitError) Is(target error) bool {
	t, ok := target.(*GitError)
	return ok && t.Code == e.Code
}

// Sentinels for errors.Is; the zero Message/ExitCode are placeholders, only Code participates in Is.
var (
	ErrNotARepository = &GitError{Code: CodeNotARepository}
	ErrNoUpstream     = &GitError{Code: CodeNoUpstream}
	ErrConflict       = &GitError{Code: CodeConflict}
	ErrAuthFailed     = &GitError{Code: CodeAuthFailed}
	ErrTimeout        = &GitError{Code: CodeTimeout}
	ErrCommandFailed  = &GitError{Code: CodeCommandFailed}
)

// classify maps a failed run's combined output to a GitError. ctx completion
// wins over git output: a killed git prints whatever it liked before dying.
func classify(ctx context.Context, output string, exitCode int) *GitError {
	if ctx.Err() != nil {
		return &GitError{Code: CodeTimeout, Message: "git killed: " + ctx.Err().Error(), ExitCode: exitCode}
	}
	message := firstLine(output)
	code := CodeCommandFailed
	switch {
	case strings.Contains(output, "not a git repository"):
		code = CodeNotARepository
	case strings.Contains(output, "no tracking information"),
		strings.Contains(output, "no upstream"):
		code = CodeNoUpstream
	case strings.Contains(output, "CONFLICT ("):
		code = CodeConflict
	case strings.Contains(output, "terminal prompts disabled"),
		strings.Contains(output, "Authentication failed"):
		code = CodeAuthFailed
	}
	return &GitError{Code: code, Message: message, ExitCode: exitCode}
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return strings.TrimRight(s[:i], "\r")
	}
	return s
}
