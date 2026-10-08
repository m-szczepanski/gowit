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

func TestFirstLineSkipsEmptyLeadingLines(t *testing.T) {
	// runGit joins stderr before stdout; when stderr is empty (git merge
	// prints everything on stdout) the join starts with a newline.
	if got := firstLine("\nAuto-merging a.txt\nCONFLICT (content)\n"); got != "Auto-merging a.txt" {
		t.Fatalf("firstLine = %q, want first non-empty line", got)
	}
	if got := firstLine("\r\nCRLF\r\n"); got != "CRLF" {
		t.Fatalf("firstLine = %q, want CRLF", got)
	}
	if got := firstLine("one\n"); got != "one" {
		t.Fatalf("single line = %q, want one", got)
	}
	if got := firstLine(""); got != "" {
		t.Fatalf("empty = %q, want empty", got)
	}
}
