package git

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func commitHead(t *testing.T, dir, format string) string {
	t.Helper()
	return gitOut(t, dir, "log", "-1", "--format="+format)
}

func gitOut(t *testing.T, dir string, args ...string) string {
	t.Helper()
	out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v: %s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

// hooksFire guards hook-dependent assertions: Git for Windows may decline
// to run POSIX hooks created by tests, which would turn rejection tests
// into successes with the wrong cause.
func hooksFire(t *testing.T, dir string, repo *Repo) {
	t.Helper()
	marker := filepath.Join(dir, "hook-probe")
	writeHook(t, dir, "pre-commit", "#!/bin/sh\ntouch '"+marker+"'\nexit 1\n")
	addFile(t, dir, "probe.txt", "x\n")
	err := repo.Commit(context.Background(), CommitOptions{Message: "probe"})
	if _, statErr := os.Stat(marker); os.IsNotExist(statErr) {
		t.Skip("git did not execute the probe hook in this environment")
	}
	if err == nil {
		t.Fatal("probe hook exited 0 but should reject")
	}
	// leave no residue: later cases assert their own clean/dirty states
	if _, rmErr := os.Stat(marker); rmErr == nil {
		_ = os.Remove(marker)
	}
	// rm --cached works on an unborn branch, restore --staged cannot
	if out, rmErr := exec.Command("git", "-C", dir, "rm", "--cached", "-q", "probe.txt").CombinedOutput(); rmErr != nil {
		t.Fatalf("unstage probe: %v: %s", rmErr, out)
	}
	if rmErr := os.Remove(filepath.Join(dir, "probe.txt")); rmErr != nil {
		t.Fatal(rmErr)
	}
	if rmErr := os.Remove(filepath.Join(dir, ".git", "hooks", "pre-commit")); rmErr != nil {
		t.Fatal(rmErr)
	}
}

func TestCommitRecordsStagedIndexWithConfiguredAuthor(t *testing.T) {
	dir := initRepo(t)
	addFile(t, dir, "a.txt", "content\n")

	repo, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	err = repo.Commit(context.Background(), CommitOptions{Message: "add a\n\nlong body here"})
	if err != nil {
		t.Fatalf("Commit: %v", err)
	}

	if got := commitHead(t, dir, "%s"); got != "add a" {
		t.Fatalf("subject = %q", got)
	}
	if got := commitHead(t, dir, "%b"); !strings.Contains(got, "long body here") {
		t.Fatalf("body = %q", got)
	}
	wantAuthor := strings.TrimSpace(gitOut(t, dir, "config", "user.name")) + " <" + strings.TrimSpace(gitOut(t, dir, "config", "user.email")) + ">"
	if got := commitHead(t, dir, "%an <%ae>"); got != wantAuthor {
		t.Fatalf("author = %q, want repo-configured identity %q", got, wantAuthor)
	}
	// the staged file is in the tree
	out, err := exec.Command("git", "-C", dir, "show", "--name-only", "--format=", "HEAD").CombinedOutput()
	if err != nil || !strings.Contains(string(out), "a.txt") {
		t.Fatalf("tree = %q err %v", out, err)
	}
}

func TestCommitAmendNoEditAndWithMessage(t *testing.T) {
	dir := initRepo(t)
	addFile(t, dir, "a.txt", "one\n")
	repo, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.Commit(context.Background(), CommitOptions{Message: "first"}); err != nil {
		t.Fatal(err)
	}
	firstOid := commitHead(t, dir, "%H")

	addFile(t, dir, "b.txt", "two\n")
	if err := repo.Commit(context.Background(), CommitOptions{Amend: true}); err != nil {
		t.Fatalf("amend no-edit: %v", err)
	}
	if got := commitHead(t, dir, "%s"); got != "first" {
		t.Fatalf("subject after amend = %q, want unchanged", got)
	}
	files, err := exec.Command("git", "-C", dir, "show", "--name-only", "--format=", "HEAD").CombinedOutput()
	if err != nil || !strings.Contains(string(files), "b.txt") {
		t.Fatalf("amended tree = %q err %v", files, err)
	}
	if commitHead(t, dir, "%H") == firstOid {
		t.Fatal("amend did not rewrite HEAD")
	}

	if err := repo.Commit(context.Background(), CommitOptions{Message: "renamed intent", Amend: true}); err != nil {
		t.Fatalf("amend with message: %v", err)
	}
	if got := commitHead(t, dir, "%s"); got != "renamed intent" {
		t.Fatalf("subject = %q, want renamed intent", got)
	}
}

func writeHook(t *testing.T, dir, name, script string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, ".git", "hooks", name), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
}

func TestCommitFailingPreCommitHook(t *testing.T) {
	dir := initRepo(t)
	repo, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	hooksFire(t, dir, repo)
	writeHook(t, dir, "pre-commit", "#!/bin/sh\necho \"lint stage 1 ok\" >&2\necho \"FAILURE: trailing problem\" >&2\nexit 1\n")
	addFile(t, dir, "a.txt", "x\n")

	var streamed []string
	err = repo.Commit(context.Background(), CommitOptions{Message: "nope", OnOutput: func(l string) { streamed = append(streamed, l) }})
	if !errors.Is(err, ErrCommitRejected) {
		t.Fatalf("err = %v, want commit_rejected", err)
	}
	if !strings.Contains(err.Error(), "FAILURE: trailing problem") {
		t.Fatalf("error should carry the hook output: %v", err)
	}
	if !strings.Contains(strings.Join(streamed, "\n"), "lint stage 1 ok") {
		t.Fatalf("OnOutput missed hook lines: %v", streamed)
	}
	if _, err := exec.Command("git", "-C", dir, "log", "-1").CombinedOutput(); err == nil {
		t.Fatal("rejected commit must not create history")
	}
}

func TestCommitFailingCommitMsgHook(t *testing.T) {
	dir := initRepo(t)
	repo, _ := Open(dir)
	hooksFire(t, dir, repo)
	writeHook(t, dir, "commit-msg", "#!/bin/sh\necho 'subject must not contain nope' >&2\nexit 1\n")
	addFile(t, dir, "a.txt", "x\n")

	err := repo.Commit(context.Background(), CommitOptions{Message: "nope"})
	if !errors.Is(err, ErrCommitRejected) {
		t.Fatalf("err = %v, want commit_rejected", err)
	}
}

func TestCommitPostCommitFailureDoesNotFailCommit(t *testing.T) {
	dir := initRepo(t)
	writeHook(t, dir, "post-commit", "#!/bin/sh\nexit 7\n")
	addFile(t, dir, "a.txt", "x\n")

	repo, _ := Open(dir)
	if err := repo.Commit(context.Background(), CommitOptions{Message: "fine"}); err != nil {
		t.Fatalf("post-commit exit must be ignored, got %v", err)
	}
	if commitHead(t, dir, "%s") != "fine" {
		t.Fatal("commit missing")
	}
}

func TestCommitEmptyMessageFailsBeforeGit(t *testing.T) {
	dir := initRepo(t)
	addFile(t, dir, "a.txt", "x\n")
	repo, _ := Open(dir)

	err := repo.Commit(context.Background(), CommitOptions{Message: "   \n  "})
	if !errors.Is(err, ErrValidationFailed) {
		t.Fatalf("err = %v, want validation_failed", err)
	}
	if !strings.Contains(err.Error(), "message required") {
		t.Fatalf("message = %v", err)
	}
	// no commit, and git was never consulted: HEAD is still unborn
	if out, err := exec.Command("git", "-C", dir, "rev-parse", "HEAD").CombinedOutput(); err == nil {
		t.Fatalf("unexpected HEAD %s", out)
	}
}

func TestCommitNothingToCommitCases(t *testing.T) {
	dir := initRepo(t)
	addFile(t, dir, "a.txt", "x\n")
	repo, _ := Open(dir)
	if err := repo.Commit(context.Background(), CommitOptions{Message: "base"}); err != nil {
		t.Fatal(err)
	}

	// fully clean tree
	err := repo.Commit(context.Background(), CommitOptions{Message: "again"})
	if !errors.Is(err, ErrNothingToCommit) {
		t.Fatalf("clean tree err = %v", err)
	}

	// dirty worktree but empty index: the "no changes added" wording
	writeFile(t, dir, "a.txt", "edited\n")
	err = repo.Commit(context.Background(), CommitOptions{Message: "again"})
	if !errors.Is(err, ErrNothingToCommit) {
		t.Fatalf("unstaged-only err = %v", err)
	}
	if !strings.Contains(err.Error(), "no changes added to commit") {
		t.Fatalf("message should carry git's own line: %v", err)
	}

	if err := repo.Commit(context.Background(), CommitOptions{Message: "empty ok", AllowEmpty: true}); err != nil {
		t.Fatalf("AllowEmpty: %v", err)
	}
	if commitHead(t, dir, "%s") != "empty ok" {
		t.Fatal("empty commit missing")
	}
}

func TestCommitHookAbortingSilentlyIsRejected(t *testing.T) {
	dir := initRepo(t)
	repo, _ := Open(dir)
	hooksFire(t, dir, repo)
	writeHook(t, dir, "pre-commit", "#!/bin/sh\nexit 1\n")
	addFile(t, dir, "a.txt", "x\n")

	err := repo.Commit(context.Background(), CommitOptions{Message: "quiet"})
	if !errors.Is(err, ErrCommitRejected) {
		t.Fatalf("err = %v, want commit_rejected", err)
	}
	if !strings.Contains(err.Error(), "without output") {
		t.Fatalf("message = %v", err)
	}
}

func TestCommitMessageIsInertArgv(t *testing.T) {
	dir := initRepo(t)
	addFile(t, dir, "a.txt", "x\n")
	repo, _ := Open(dir)

	evil := "msg with $(touch pwned) `touch pwned2` ; echo hi"
	if err := repo.Commit(context.Background(), CommitOptions{Message: evil}); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	if got := commitHead(t, dir, "%s"); got != evil {
		t.Fatalf("subject = %q, want the literal message", got)
	}
	for _, victim := range []string{"pwned", "pwned2"} {
		if _, err := os.Stat(filepath.Join(dir, victim)); err == nil {
			t.Fatalf("%s created: message reached a shell", victim)
		}
	}
}

func TestCommitAmendWithoutHistoryStaysCommandFailed(t *testing.T) {
	dir := initRepo(t)
	addFile(t, dir, "a.txt", "x\n")
	repo, _ := Open(dir)

	err := repo.Commit(context.Background(), CommitOptions{Message: "x", Amend: true})
	var ge *GitError
	if !errors.As(err, &ge) || ge.Code != CodeCommandFailed {
		t.Fatalf("err = %v, want command_failed for amend without HEAD", err)
	}
	if !strings.Contains(err.Error(), "fatal:") {
		t.Fatalf("tail should carry git's explanation: %v", err)
	}
}

func TestCommitTailTrimKeepsRecentHookOutput(t *testing.T) {
	dir := initRepo(t)
	repo, _ := Open(dir)
	hooksFire(t, dir, repo)
	writeHook(t, dir, "pre-commit", "#!/bin/sh\nfor i in $(seq 1 40); do echo \"line $i\" >&2; done\nexit 1\n")
	addFile(t, dir, "a.txt", "x\n")

	var seen int
	err := repo.Commit(context.Background(), CommitOptions{Message: "x", OnOutput: func(string) { seen++ }})
	if !errors.Is(err, ErrCommitRejected) {
		t.Fatalf("err = %v", err)
	}
	if seen < 40 {
		t.Fatalf("OnOutput saw %d lines, hook printed 40", seen)
	}
	if strings.Contains(err.Error(), "line 1\n") {
		t.Fatal("early output line survived; want only the tail")
	}
	if !strings.Contains(err.Error(), "line 40") {
		t.Fatalf("tail missing: %v", err)
	}
}

func TestCommitContextCancelReportsTimeout(t *testing.T) {
	dir := initRepo(t)
	addFile(t, dir, "a.txt", "x\n")
	repo, _ := Open(dir)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := repo.Commit(ctx, CommitOptions{Message: "never"})
	if !errors.Is(err, ErrTimeout) {
		t.Fatalf("err = %v, want timeout pass-through", err)
	}
}

func TestClassifyCommitErrorPassesThroughNonGitErrors(t *testing.T) {
	tail := newOutputTail(4)
	tail.add("boom")
	if err := classifyCommitError(errors.New("plain"), tail); err == nil || err.Error() != "plain" {
		t.Fatalf("err = %v, want untouched passthrough", err)
	}
}

func TestLastNonEmptyLine(t *testing.T) {
	if got := lastNonEmptyLine("a\nb\n\n"); got != "b" {
		t.Fatalf("got %q", got)
	}
	if got := lastNonEmptyLine(""); got != "" {
		t.Fatalf("got %q", got)
	}
}

func TestCommitBothStreamsRace(t *testing.T) {
	// the hook hammers stdout and stderr concurrently so the two copier
	// goroutines genuinely interleave; -race must stay quiet because
	// Commit serializes onLine under its lock
	dir := initRepo(t)
	repo, _ := Open(dir)
	hooksFire(t, dir, repo)
	writeHook(t, dir, "pre-commit", "#!/bin/sh\nfor i in $(seq 1 200); do echo out $i; echo err $i >&2; done\nexit 1\n")
	addFile(t, dir, "a.txt", "x\n")

	var localMu sync.Mutex
	count := 0
	err := repo.Commit(context.Background(), CommitOptions{Message: "race", OnOutput: func(string) {
		localMu.Lock()
		count++
		localMu.Unlock()
	}})
	if !errors.Is(err, ErrCommitRejected) {
		t.Fatalf("err = %v", err)
	}
	if count < 400 {
		t.Fatalf("OnOutput saw %d lines, want ~400 from both streams", count)
	}
}
