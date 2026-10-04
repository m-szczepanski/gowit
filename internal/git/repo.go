// Package git wraps the system git CLI. All operations shell out to
// `git` (no cgo / libgit2) for behavioural parity with the terminal —
// see docs/ARCHITECTURE.md §3.
package git

import (
	"context"
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
	// git reports a missing repository with exit 128 and a "not a git
	// repository" fatal, which classify maps to ErrNotARepository.
	if _, _, err := runGit(context.Background(), abs, "rev-parse", "--is-inside-work-tree"); err != nil {
		return nil, err
	}
	return &Repo{path: abs}, nil
}

// Path returns the absolute path to the repository working tree.
func (r *Repo) Path() string {
	return r.path
}

// Close is a placeholder: no resources are held yet. Watchers and
// in-flight git commands will register their cleanup here.
func (r *Repo) Close() error {
	return nil
}
