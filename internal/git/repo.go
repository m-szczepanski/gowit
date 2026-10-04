// Package git wraps the system git CLI. All operations shell out to
// `git` (no cgo / libgit2) for behavioural parity with the terminal —
// see docs/ARCHITECTURE.md §3.
package git

import (
	"fmt"
	"os/exec"
	"path/filepath"
)

// Repo represents an open git repository on disk.
type Repo struct {
	path string
}

// Open verifies that path is a git work tree and returns a Repo for it.
func Open(path string) (*Repo, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("resolve path %q: %w", path, err)
	}
	if _, err := exec.LookPath("git"); err != nil {
		return nil, fmt.Errorf("git executable not found on PATH: %w", err)
	}
	out, err := exec.Command("git", "-C", abs, "rev-parse", "--is-inside-work-tree").Output()
	if err != nil || string(out) != "true\n" {
		return nil, fmt.Errorf("%q is not a git repository", abs)
	}
	return &Repo{path: abs}, nil
}

// Path returns the absolute path to the repository working tree.
func (r *Repo) Path() string {
	return r.path
}

// Close releases any resources held by the Repo. It is currently a
// placeholder for future cleanup (watchers, in-flight commands).
func (r *Repo) Close() error {
	return nil
}
