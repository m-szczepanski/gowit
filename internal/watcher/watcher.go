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
	"time"

	"github.com/fsnotify/fsnotify"
)

const (
	defaultDebounce = 300 * time.Millisecond
	// defaultMaxWatchers keeps one process well under the usual inotify
	// instance limit (8192) with headroom for the user's other watchers.
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

	fsw     *fsnotify.Watcher
	done    chan struct{}
	closed  chan struct{}
	used    int
	polling bool
}

// New prepares a watcher for the work tree at root. Nothing runs until
// Start.
func New(root string, onChange func(), opts Options) *Watcher {
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
func (w *Watcher) Start(ctx context.Context) error {
	fsw, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	w.fsw = fsw
	w.done = make(chan struct{})
	w.closed = make(chan struct{})
	w.ctx = ctx

	addErr := w.addTree(ctx, w.root)
	if addErr != nil && !errors.Is(addErr, errBudget) {
		_ = fsw.Close()
		return addErr
	}
	w.addGitWatches()
	if errors.Is(addErr, errBudget) || w.used >= w.budget {
		w.demote()
	}
	go w.run(ctx)
	return nil
}

// Stop ends delivery and releases OS handles. Safe to call twice; returns
// after the watch loop has exited.
func (w *Watcher) Stop() error {
	if w.done == nil {
		return nil
	}
	select {
	case <-w.closed:
		return nil
	default:
	}
	close(w.done)
	<-w.closed
	return nil
}

func (w *Watcher) addWatch(path string) error {
	if w.used >= w.budget {
		w.demote()
		return errBudget
	}
	if err := w.fsw.Add(path); err != nil {
		return err
	}
	w.used++
	return nil
}

// demote starts the periodic re-scan exactly once.
func (w *Watcher) demote() {
	if w.polling {
		return
	}
	w.polling = true
	go w.poll()
}

func (w *Watcher) poll() {
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
				w.onChange()
			}
		}
	}
}

// signature fingerprints everything a signal depends on: paths, sizes and
// mtimes across the work tree and .git state, with git noise skipped.
func (w *Watcher) signature() string {
	var b strings.Builder
	err := filepath.WalkDir(w.root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if w.ignoredGitNoise(path) {
			// noise only ever appears as a directory: its parents are
			// visited first and skipped wholesale
			return filepath.SkipDir
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		fmt.Fprintf(&b, "%s|%d|%d\n", path, info.Size(), info.ModTime().UnixNano())
		return nil
	})
	if err != nil {
		return "walk-error"
	}
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
		if e := w.addWatch(path); e == errBudget {
			budgetErr = errBudget
			return filepath.SkipAll
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
	defer close(w.closed)
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
		case <-timer.C:
			w.onChange()
		}
	}
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
