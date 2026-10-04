package git

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func commitRepo(t *testing.T, dir, msg string) {
	t.Helper()
	if _, _, err := runGit(context.Background(), dir,
		"-c", "user.email=t@t", "-c", "user.name=t", "commit", "--allow-empty", "-m", msg); err != nil {
		t.Fatal(err)
	}
}

func TestLineSplitterBuffersPartialLines(t *testing.T) {
	var lines []string
	w := &lineSplitter{onLine: func(l string) { lines = append(lines, l) }}

	n, err := w.Write([]byte("one\ntw"))
	if n != 6 || err != nil {
		t.Fatalf("Write = (%d, %v)", n, err)
	}
	if strings.Join(lines, "|") != "one" {
		t.Fatalf("lines = %q, want [one]", lines)
	}

	w.Write([]byte("o\n"))
	w.flush()
	if strings.Join(lines, "|") != "one|two" {
		t.Fatalf("lines = %q, want [one two]", lines)
	}

	w.Write([]byte("tail"))
	w.flush()
	if strings.Join(lines, "|") != "one|two|tail" {
		t.Fatalf("lines = %q, want trailing fragment flushed", lines)
	}
}

func TestRunGitStreamEmitsStdoutLines(t *testing.T) {
	dir := initRepo(t)
	commitRepo(t, dir, "first")
	commitRepo(t, dir, "second")

	var lines []string
	err := runGitStream(context.Background(), dir, func(line string) { lines = append(lines, line) },
		"log", "--pretty=%s")
	if err != nil {
		t.Fatalf("runGitStream: %v", err)
	}
	if strings.Join(lines, "\n") != "second\nfirst" {
		t.Fatalf("lines = %q, want [second first] (newest first)", lines)
	}
}

func TestRunGitStreamFeedsStderrProgressAndMapsFailure(t *testing.T) {
	seen := false
	err := runGitStream(context.Background(), t.TempDir(), func(line string) {
		if strings.Contains(line, "not a git repository") {
			seen = true
		}
	}, "rev-parse", "--is-inside-work-tree")

	if !errors.Is(err, ErrNotARepository) {
		t.Fatalf("err = %v, want ErrNotARepository", err)
	}
	if !seen {
		t.Fatal("stderr lines must reach the callback (fetch/push progress lives there)")
	}
}
