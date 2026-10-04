// Package watcher detects working-tree and repository changes so the UI
// refreshes without a manual poll. Strategy per ARCHITECTURE.md §7:
// fsnotify over the work tree, events debounced (~300ms) to survive
// bursts like npm install or checkout, and .git/ ignored except the
// files that signal repo state: .git/HEAD, .git/refs/, .git/index.
package watcher

// Watcher is an empty placeholder; nothing is watched yet.
type Watcher struct{}
