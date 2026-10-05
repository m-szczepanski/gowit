package watcher

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

// signals collects onChange invocations.
func signalChan(t *testing.T) (func(), chan struct{}) {
	t.Helper()
	ch := make(chan struct{}, 64)
	return func() {
		select {
		case ch <- struct{}{}:
		default:
		}
	}, ch
}

func waitFor(t *testing.T, ch chan struct{}, within time.Duration) bool {
	t.Helper()
	select {
	case <-ch:
		return true
	case <-time.After(within):
		return false
	}
}

func noSignal(t *testing.T, ch chan struct{}, quiet time.Duration) {
	t.Helper()
	select {
	case <-ch:
		t.Fatal("unexpected signal")
	case <-time.After(quiet):
	}
}

func TestStartEmitsOnWorktreeChange(t *testing.T) {
	dir := t.TempDir()
	onChange, ch := signalChan(t)

	w := New(dir, onChange, Options{Debounce: 10 * time.Millisecond})
	ctx := context.Background()
	if err := w.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer func() { _ = w.Stop() }()

	if err := os.WriteFile(filepath.Join(dir, "new.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !waitFor(t, ch, 2*time.Second) {
		t.Fatal("no signal after worktree create")
	}
}

func TestBurstProducesSingleDebouncedSignal(t *testing.T) {
	dir := t.TempDir()
	onChange, ch := signalChan(t)

	w := New(dir, onChange, Options{Debounce: 30 * time.Millisecond})
	if err := w.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = w.Stop() }()

	for i := 0; i < 20; i++ {
		name := filepath.Join(dir, "f"+string(rune('a'+i%26))+".txt")
		switch i % 3 {
		case 0:
			_ = os.WriteFile(name, []byte("x"), 0o644)
		case 1:
			_ = os.WriteFile(name, []byte("y"), 0o644)
		case 2:
			_ = os.Remove(name)
		}
	}

	if !waitFor(t, ch, 2*time.Second) {
		t.Fatal("no signal after burst")
	}
	noSignal(t, ch, 300*time.Millisecond)
}

func TestStopHaltsDeliveryAndIsIdempotent(t *testing.T) {
	dir := t.TempDir()
	onChange, ch := signalChan(t)
	w := New(dir, onChange, Options{Debounce: 10 * time.Millisecond})
	if err := w.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := w.Stop(); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	if err := w.Stop(); err != nil {
		t.Fatalf("second Stop: %v", err)
	}

	if err := os.WriteFile(filepath.Join(dir, "after.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	noSignal(t, ch, 200*time.Millisecond)
}

func initGitRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t",
			"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
	}
	run("-c", "init.defaultBranch=main", "init")
	run("config", "core.autocrlf", "false")
	return dir
}

func TestGitStateFilesSignal(t *testing.T) {
	for _, f := range []string{"HEAD", "index", "packed-refs"} {
		t.Run(f, func(t *testing.T) {
			dir := initGitRepo(t)
			if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("a\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			runGitIn(t, dir, "add", "a.txt")
			runGitIn(t, dir, "commit", "-qm", "c")

			onChange, ch := signalChan(t)
			w := New(dir, onChange, Options{Debounce: 20 * time.Millisecond})
			if err := w.Start(context.Background()); err != nil {
				t.Fatal(err)
			}
			defer func() { _ = w.Stop() }()

			p := filepath.Join(dir, ".git", f)
			if f == "packed-refs" {
				// absent in a fresh repo; its creation must count
				if err := os.WriteFile(p, []byte("# pack-refs\n"), 0o644); err != nil {
					t.Fatal(err)
				}
			} else if err := os.WriteFile(p, append(mustRead(t, p), ' '), 0o644); err != nil {
				t.Fatal(err)
			}
			if !waitFor(t, ch, 2*time.Second) {
				t.Fatalf("no signal for .git/%s change", f)
			}
		})
	}
}

func TestRefsUpdateSignals(t *testing.T) {
	dir := initGitRepo(t)
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGitIn(t, dir, "add", "a.txt")
	runGitIn(t, dir, "commit", "-qm", "c")

	onChange, ch := signalChan(t)
	w := New(dir, onChange, Options{Debounce: 20 * time.Millisecond})
	if err := w.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = w.Stop() }()

	runGitIn(t, dir, "branch", "feature")
	if !waitFor(t, ch, 2*time.Second) {
		t.Fatal("no signal for new ref")
	}
}

func TestRebaseMarkerDirectorySignals(t *testing.T) {
	dir := initGitRepo(t)
	onChange, ch := signalChan(t)
	w := New(dir, onChange, Options{Debounce: 20 * time.Millisecond})
	if err := w.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = w.Stop() }()

	if err := os.Mkdir(filepath.Join(dir, ".git", "rebase-merge"), 0o755); err != nil {
		t.Fatal(err)
	}
	if !waitFor(t, ch, 2*time.Second) {
		t.Fatal("no signal for rebase-merge marker")
	}
}

func TestGitNoiseDoesNotSignal(t *testing.T) {
	dir := initGitRepo(t)
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGitIn(t, dir, "add", "a.txt")
	runGitIn(t, dir, "commit", "-qm", "c")

	onChange, ch := signalChan(t)
	w := New(dir, onChange, Options{Debounce: 20 * time.Millisecond})
	if err := w.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = w.Stop() }()

	objects := filepath.Join(dir, ".git", "objects", "ab")
	if err := os.MkdirAll(objects, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(objects, "cdef"), []byte("blob"), 0o644); err != nil {
		t.Fatal(err)
	}
	logs := filepath.Join(dir, ".git", "logs")
	if err := os.WriteFile(filepath.Join(logs, "HEAD"), []byte("reflog noise\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".git", "hooks", "pre-commit"), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	noSignal(t, ch, 400*time.Millisecond)
}

func runGitIn(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t",
		"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v: %s", args, err, out)
	}
}

func mustRead(t *testing.T, p string) []byte {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestDirectoryCreatedAfterStartIsWatched(t *testing.T) {
	dir := t.TempDir()
	onChange, ch := signalChan(t)
	w := New(dir, onChange, Options{Debounce: 20 * time.Millisecond})
	if err := w.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = w.Stop() }()

	sub := filepath.Join(dir, "src", "deep")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	if !waitFor(t, ch, 2*time.Second) {
		t.Fatal("no signal for new directory")
	}

	// the mkdir signal only proves the root watch fired; this next write is
	// invisible unless src/deep itself got registered on the fly
	time.Sleep(200 * time.Millisecond)
	if err := os.WriteFile(filepath.Join(sub, "leaf.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !waitFor(t, ch, 2*time.Second) {
		t.Fatal("no signal for file inside directory created after Start")
	}
}

func TestGitInitAfterStartBeginsStateWatches(t *testing.T) {
	dir := t.TempDir()
	onChange, ch := signalChan(t)
	w := New(dir, onChange, Options{Debounce: 20 * time.Millisecond})
	if err := w.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = w.Stop() }()

	cmd := exec.Command("git", "-C", dir, "init")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init: %v: %s", err, out)
	}
	if !waitFor(t, ch, 2*time.Second) {
		t.Fatal("no signal for in-place git init")
	}
	time.Sleep(200 * time.Millisecond)

	// HEAD writes must now be watched even though .git did not exist at Start
	if err := os.WriteFile(filepath.Join(dir, ".git", "HEAD"), []byte("ref: refs/heads/main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !waitFor(t, ch, 2*time.Second) {
		t.Fatal("no signal for HEAD write after late .git")
	}
}

func TestNewRefsSubdirectoryWatched(t *testing.T) {
	dir := initGitRepo(t)
	onChange, ch := signalChan(t)
	w := New(dir, onChange, Options{Debounce: 20 * time.Millisecond})
	if err := w.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = w.Stop() }()

	custom := filepath.Join(dir, ".git", "refs", "custom")
	if err := os.Mkdir(custom, 0o755); err != nil {
		t.Fatal(err)
	}
	if !waitFor(t, ch, 2*time.Second) {
		t.Fatal("no signal for new refs subdir")
	}
	time.Sleep(200 * time.Millisecond)
	if err := os.WriteFile(filepath.Join(custom, "ref"), []byte("abc\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !waitFor(t, ch, 2*time.Second) {
		t.Fatal("no signal inside new refs subdir")
	}
}

func TestFallsBackToPollingWhenBudgetExhausted(t *testing.T) {
	dir := initGitRepo(t)
	// two subdirs guarantee the walk itself hits the budget mid-tree
	for _, d := range []string{"src", "src/deep"} {
		if err := os.Mkdir(filepath.Join(dir, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	onChange, ch := signalChan(t)
	w := New(dir, onChange, Options{
		Debounce:    20 * time.Millisecond,
		MaxWatchers: 1,
		PollEvery:   50 * time.Millisecond,
	})
	if err := w.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = w.Stop() }()

	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !waitFor(t, ch, 2*time.Second) {
		t.Fatal("poll fallback produced no signal for worktree change")
	}
	// the registered worktree watch and the poller may each report the same
	// burst; consumers are idempotent, so drain before asserting silence
	for drained := false; !drained; {
		select {
		case <-ch:
		case <-time.After(300 * time.Millisecond):
			drained = true
		}
	}

	noise := filepath.Join(dir, ".git", "objects", "cd")
	if err := os.MkdirAll(noise, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(noise, "ef"), []byte("b"), 0o644); err != nil {
		t.Fatal(err)
	}
	noSignal(t, ch, 400*time.Millisecond)
}

func TestDefaultDebounceWhenOptionsEmpty(t *testing.T) {
	dir := t.TempDir()
	onChange, ch := signalChan(t)
	w := New(dir, onChange, Options{})
	defer func() { _ = w.Stop() }()

	if err := w.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "x.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	// 300ms default must still land well inside the wait window
	if !waitFor(t, ch, 3*time.Second) {
		t.Fatal("default debounce never fired")
	}
}

func TestStopBeforeStart(t *testing.T) {
	w := New(t.TempDir(), func() {}, Options{})
	if err := w.Stop(); err != nil {
		t.Fatalf("Stop on fresh watcher: %v", err)
	}
}

func TestStartFailsWithCancelledContext(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "sub")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	w := New(dir, func() {}, Options{MaxWatchers: 2})
	err := w.Start(ctx)
	if err == nil {
		_ = w.Stop()
		t.Fatal("want error from Start with dead ctx")
	}
	// second Start attempt must fail cleanly too, with budget exhausted path
	w2 := New(dir, func() {}, Options{})
	if err := w2.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	_ = w2.Stop()
}

func TestContextCancelStopsDelivery(t *testing.T) {
	dir := t.TempDir()
	onChange, ch := signalChan(t)
	ctx, cancel := context.WithCancel(context.Background())
	w := New(dir, onChange, Options{Debounce: 10 * time.Millisecond})
	if err := w.Start(ctx); err != nil {
		t.Fatal(err)
	}
	cancel()
	// run loop exits on ctx; give it a moment, then writes must be silent
	time.Sleep(100 * time.Millisecond)
	if err := os.WriteFile(filepath.Join(dir, "late.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	noSignal(t, ch, 300*time.Millisecond)
	_ = w.Stop()
}

func TestPollExitsOnContextCancel(t *testing.T) {
	dir := t.TempDir()
	onChange, ch := signalChan(t)
	ctx, cancel := context.WithCancel(context.Background())
	w := New(dir, onChange, Options{MaxWatchers: 1, PollEvery: 30 * time.Millisecond})
	if err := w.Start(ctx); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !waitFor(t, ch, 2*time.Second) {
		t.Fatal("poller never fired")
	}
	cancel()
	time.Sleep(100 * time.Millisecond)
	// after cancel the walk error path would otherwise keep signalling on
	// any further tree change; make sure nothing arrives anymore
	if err := os.WriteFile(filepath.Join(dir, "b.txt"), []byte("y"), 0o644); err != nil {
		t.Fatal(err)
	}
	noSignal(t, ch, 300*time.Millisecond)
	_ = w.Stop()
}

func TestPollSurvivesVanishedRoot(t *testing.T) {
	dir := t.TempDir()
	onChange, ch := signalChan(t)
	w := New(dir, onChange, Options{MaxWatchers: 1, PollEvery: 30 * time.Millisecond})
	if err := w.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = w.Stop() }()

	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !waitFor(t, ch, 2*time.Second) {
		t.Fatal("poller never fired")
	}
	drainQuiet(t, ch)

	if err := os.RemoveAll(dir); err != nil {
		t.Fatal(err)
	}
	if !waitFor(t, ch, 2*time.Second) {
		t.Fatal("vanishing the root must produce one last signal")
	}
	noSignal(t, ch, 200*time.Millisecond)
}

func drainQuiet(t *testing.T, ch chan struct{}) {
	t.Helper()
	for {
		select {
		case <-ch:
		case <-time.After(250 * time.Millisecond):
			return
		}
	}
}

func TestClosingEventSourceEndsRunLoop(t *testing.T) {
	dir := t.TempDir()
	w := New(dir, func() {}, Options{})
	if err := w.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	w.fsw.Close()
	// run must observe the closed Events channel and exit; Stop then finds
	// the loop already terminated and returns cleanly
	done := make(chan struct{})
	go func() { _ = w.Stop(); close(done) }()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("run loop did not exit after event source closed")
	}
}
