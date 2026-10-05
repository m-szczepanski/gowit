// Package git wraps the system git CLI. All operations shell out to
// `git` (no cgo / libgit2) for behavioural parity with the terminal —
// see docs/ARCHITECTURE.md §3.
package git

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Repo represents an open git repository on disk. Path is always the
// work-tree root, so opening a subdirectory or a linked worktree binds the
// whole repository. Submodule checkouts open as ordinary repositories.
type Repo struct {
	path string
}

// Open validates that path is inside a git work tree and returns a Repo
// rooted at its top level.
func Open(path string) (*Repo, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("resolve path %q: %w", path, err)
	}
	if _, err := os.Stat(abs); err != nil {
		return nil, &GitError{
			Code:     CodePathMissing,
			Message:  fmt.Sprintf("%s does not exist", abs),
			ExitCode: -1,
		}
	}
	if _, err := exec.LookPath("git"); err != nil {
		return nil, fmt.Errorf("git executable not found on PATH: %w", err)
	}

	ctx := context.Background()
	out, stderr, err := runGit(ctx, abs, "rev-parse", "--show-toplevel")
	if err == nil {
		// git always prints forward slashes; Clean canonicalizes to the
		// native separator so Repo paths match Go paths and recents dedupe
		return &Repo{path: filepath.Clean(strings.TrimSpace(string(out)))}, nil
	}

	if bare, _, bareErr := runGit(ctx, abs, "rev-parse", "--is-bare-repository"); bareErr == nil &&
		strings.TrimSpace(string(bare)) == "true" {
		return nil, &GitError{
			Code:     CodeBareRepository,
			Message:  fmt.Sprintf("%s is a bare repository with no working tree; clone it first", abs),
			ExitCode: exitCodeOf(err),
		}
	}

	gitErr := classify(ctx, string(stderr)+"\n"+string(out), exitCodeOf(err))
	if gitErr.Code == CodeNotARepository {
		if children := childRepos(abs); len(children) > 0 {
			gitErr.Message = childReposHint(abs, children)
		}
	}
	return nil, gitErr
}

func childReposHint(abs string, children []string) string {
	if len(children) == 1 {
		return fmt.Sprintf("%s is not a repository, but %s is - open it", abs, children[0])
	}
	return fmt.Sprintf("%s is not a repository, but %s are - open one of them", abs, strings.Join(children, ", "))
}

func childRepos(dir string) []string {
	// a ReadDir failure just yields no entries: no hint, same outcome
	entries, _ := os.ReadDir(dir)
	var found []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		child := filepath.Join(dir, e.Name())
		if _, err := os.Stat(filepath.Join(child, ".git")); err == nil {
			found = append(found, child)
		}
	}
	return found
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
