package git

import (
	"errors"
	"testing"
)

func TestGitErrorFormatAndIs(t *testing.T) {
	err := &GitError{Code: CodeNoUpstream, Message: "no tracking information", ExitCode: 1}

	if want := "git no_upstream: no tracking information (exit 1)"; err.Error() != want {
		t.Fatalf("Error() = %q, want %q", err.Error(), want)
	}
	if !errors.Is(err, ErrNoUpstream) {
		t.Fatal("want errors.Is against ErrNoUpstream sentinel")
	}
	if errors.Is(err, ErrTimeout) {
		t.Fatal("must not match a different code")
	}
	if !errors.Is(error(err), err) {
		t.Fatal("a sentinel must match its own code")
	}
}
