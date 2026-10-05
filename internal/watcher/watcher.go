// Package watcher detects working-tree and repository changes so the UI
// refreshes without a manual poll. Strategy per ARCHITECTURE.md §7:
// fsnotify over the work tree, events debounced (~300ms) to survive
// bursts like npm install or checkout, and .git/ ignored except the
// files that signal repo state: .git/HEAD, .git/refs/, .git/index.
package watcher

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

const (
	defaultDebounce = 300 * time.Millisecond
	// defaultMaxWatchers bounds one process on any platform: well under
	// Linux's default inotify instance limit (8192) and cheap everywhere
	// else (kqueue/ReadDirectoryChangesW scale with open handles).
	defaultMaxWatchers = 1000
	defaultPollEvery   = 2 * time.Second
)

var errBudget = errors.New("watcher budget exhausted")

// Options tunes the watcher. Zero values select the defaults; tests lower
// Debounce to keep runs fast without changing behavior.
type Options struct {
	Debounce time.Duration
	// MaxWatchers caps registered paths. When the cap is hit the watcher
	// degrades: existing event watches continue and a periodic re-scan of
	// the whole tree (work tree plus .git state paths) starts on PollEvery,
	// so subtrees that never got a watch still produce signals.
	MaxWatchers int
	PollEvery   time.Duration
}

// Watcher emits one coarse onChange signal per quiet burst of filesystem
// activity. Start owns a goroutine until Stop.
type Watcher struct {
	root      string
	gitDir    string
	onChange  func()
	debounce  time.Duration
	budget    int
	pollEvery time.Duration
	ctx       context.Context

	fsw      *fsnotify.Watcher
	done     chan struct{}
	stopOnce sync.Once
	wg       sync.WaitGroup
	polling  bool
}

// New prepares a watcher for the work tree at root. Nothing runs until
// Start.
func New(root string, onChange func(), opts Options) *Watcher {
	if onChange == nil {
		onChange = func() {}
	}
	d := opts.Debounce
	if d <= 0 {
		d = defaultDebounce
	}
	b := opts.MaxWatchers
	if b <= 0 {
		b = defaultMaxWatchers
	}
	p := opts.PollEvery
	if p <= 0 {
		p = defaultPollEvery
	}
	return &Watcher{
		root:      root,
		gitDir:    filepath.Join(root, ".git"),
		onChange:  onChange,
		debounce:  d,
		budget:    b,
		pollEvery: p,
	}
}

// Start watches root recursively and delivers debounced signals until the
// context is cancelled or Stop is called.
// Start registers watches and begins delivery. One Watcher instance runs a
// single Start/Stop cycle; reuse after Stop or a failed Start means a new
// New().
func (w *Watcher) Start(ctx context.Context) error {
	if w.fsw != nil {
		return errors.New("watcher: already started")
	}
	fsw, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	w.fsw = fsw
	w.done = make(chan struct{})
	w.ctx = ctx

	addErr := w.addTree(ctx, w.root)
	if addErr != nil && !errors.Is(addErr, errBudget) {
		// ctx already dead: hand back a pristine watcher; Stop must not
		// wait for a loop that never started
		_ = fsw.Close()
		w.fsw, w.done = nil, nil
		return addErr
	}
	w.addGitWatches()
	if errors.Is(addErr, errBudget) || len(w.fsw.WatchList()) >= w.budget {
		w.demote()
	}
	w.wg.Add(1)
	go w.run(ctx)
	return nil
}

// Stop ends delivery and releases OS handles. Safe to call concurrently or
// twice; returns only after neither the event loop nor the poller can emit
// again.
func (w *Watcher) Stop() error {
	if w.done == nil {
		return nil
	}
	w.stopOnce.Do(func() { close(w.done) })
	w.wg.Wait()
	return nil
}

// addWatch registers one path. Budget exhaustion or an OS failure leaves
// that path unwatched; the poll fallback takes over so no gap stays dark.
func (w *Watcher) addWatch(path string) error {
	// WatchList reflects implicit removals (deleted directories), so the
	// budget tracks live handles rather than lifetime registrations
	if len(w.fsw.WatchList()) >= w.budget {
		w.demote()
		return errBudget
	}
	if err := w.fsw.Add(path); err != nil {
		w.demote()
		return err
	}
	return nil
}

// demote starts the periodic re-scan exactly once.
func (w *Watcher) demote() {
	if w.polling {
		return
	}
	w.polling = true
	w.wg.Add(1)
	go w.poll()
}

func (w *Watcher) poll() {
	defer w.wg.Done()
	ticker := time.NewTicker(w.pollEvery)
	defer ticker.Stop()
	prev := w.signature()
	for {
		select {
		case <-w.ctx.Done():
			return
		case <-w.done:
			return
		case <-ticker.C:
			if sig := w.signature(); sig != prev {
				prev = sig
				w.emit()
			}
		}
	}
}

// signature fingerprints everything a signal depends on: paths, sizes,
// mtimes and modes across the work tree and .git state, with git noise
// skipped. On filesystems with coarse mtime granularity (FAT/exFAT) a
// same-size edit inside one clock tick can go unnoticed; accepted for the
// degraded mode.
func (w *Watcher) signature() string {
	var b strings.Builder
	_ = filepath.WalkDir(w.root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if w.ignoredGitNoise(path) {
			// SkipDir from a file entry would blind the walk to its
			// siblings, so only directories skip
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		fmt.Fprintf(&b, "%s|%d|%d|%s\n", path, info.Size(), info.ModTime().UnixNano(), info.Mode())
		return nil
	})
	return b.String()
}

// addTree registers root and every directory below it, skipping .git.
func (w *Watcher) addTree(ctx context.Context, root string) error {
	var budgetErr error
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if !d.IsDir() {
			return nil
		}
		if path != root && d.Name() == ".git" {
			return filepath.SkipDir
		}
		if e := w.addWatch(path); e != nil {
			if errors.Is(e, errBudget) {
				budgetErr = errBudget
				return filepath.SkipAll
			}
			// per-directory OS failure: demoted already, keep walking
		}
		return nil
	})
	if err != nil {
		return err
	}
	return budgetErr
}

// addGitWatches registers the .git state sources: the top-level .git
// directory (marker files, packed-refs, index swaps), the HEAD and index
// files themselves, and everything under refs/. Deeper .git subtrees
// (objects, logs, hooks) are never registered, so their churn produces no
// events at all.
func (w *Watcher) addGitWatches() {
	// linked worktrees store .git as a file pointing at a gitdir elsewhere;
	// resolving that is future work, so those checkouts currently watch
	// only the work tree
	if st, err := os.Stat(w.gitDir); err != nil || !st.IsDir() {
		return
	}
	_ = w.addWatch(w.gitDir)
	for _, f := range []string{"HEAD", "index"} {
		_ = w.addWatch(filepath.Join(w.gitDir, f))
	}
	refs := filepath.Join(w.gitDir, "refs")
	if st, err := os.Stat(refs); err == nil && st.IsDir() {
		_ = filepath.WalkDir(refs, func(path string, d fs.DirEntry, err error) error {
			if err == nil && d.IsDir() {
				_ = w.addWatch(path)
			}
			return nil
		})
	}
}

func (w *Watcher) relevant(ev fsnotify.Event) bool {
	return !w.ignoredGitNoise(ev.Name)
}

func (w *Watcher) run(ctx context.Context) {
	defer w.wg.Done()
	defer func() { _ = w.fsw.Close() }()

	timer := time.NewTimer(time.Hour)
	timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-w.done:
			return
		case ev, ok := <-w.fsw.Events:
			if !ok {
				return
			}
			if w.relevant(ev) {
				timer.Reset(w.debounce)
			}
			if ev.Op&fsnotify.Create != 0 {
				w.registerNewDir(ev.Name)
			}
		case err, ok := <-w.fsw.Errors:
			// fsnotify's backends send on Errors inline and block until
			// it is drained; ignoring the channel wedges the stream.
			// Queue overflow is exactly the event flood the poller exists
			// for, so it engages the fallback.
			if !ok {
				return
			}
			if errors.Is(err, fsnotify.ErrEventOverflow) {
				w.demote()
			}
		case <-timer.C:
			w.emit()
		}
	}
}

// emit delivers the signal unless Stop began concurrently; the watcher
// never calls back into App after Stop returned.
func (w *Watcher) emit() {
	select {
	case <-w.done:
		return
	default:
	}
	w.onChange()
}

// registerNewDir brings directories created after Start under watch, so
// later activity inside them is not lost. Budget exhaustion is fine: the
// poll fallback covers whatever could not be registered.
func (w *Watcher) registerNewDir(path string) {
	if w.ignoredGitNoise(path) {
		return
	}
	st, err := os.Stat(path)
	if err != nil || !st.IsDir() {
		return
	}
	if rel, err := filepath.Rel(w.gitDir, path); err == nil && rel == "." {
		w.addGitWatches()
		return
	}
	if insideGit(w.gitDir, path) {
		_ = w.addWatch(path)
		return
	}
	_ = w.addTree(w.ctx, path)
}

func (w *Watcher) ignoredGitNoise(path string) bool {
	rel, err := filepath.Rel(w.gitDir, path)
	if err != nil || rel == "." || strings.HasPrefix(rel, "..") {
		return false
	}
	first := strings.SplitN(filepath.ToSlash(rel), "/", 2)[0]
	for _, n := range gitNoiseDirs {
		if first == n {
			return true
		}
	}
	return false
}

var gitNoiseDirs = []string{"objects", "logs", "hooks", "info"}

func insideGit(gitDir, path string) bool {
	rel, err := filepath.Rel(gitDir, path)
	return err == nil && rel != "." && !strings.HasPrefix(rel, "..")
}
