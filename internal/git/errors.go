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

// classify maps a failed run to a GitError. ctx completion wins over stderr
// text: a killed git prints whatever it liked before dying.
func classify(ctx context.Context, stderr string, exitCode int) *GitError {
	message := firstLine(stderr)
	if ctx.Err() != nil {
		return &GitError{Code: CodeTimeout, Message: "git killed: " + ctx.Err().Error(), ExitCode: exitCode}
	}
	switch {
	case strings.Contains(stderr, "not a git repository"):
		return &GitError{Code: CodeNotARepository, Message: message, ExitCode: exitCode}
	}
	return &GitError{Code: CodeCommandFailed, Message: message, ExitCode: exitCode}
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return strings.TrimRight(s[:i], "\r")
	}
	return s
}
