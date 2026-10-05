package git

import (
	"context"
	"slices"
	"strings"
)

// Paths travel as argv after a literal "--": no shell and no glob
// expansion, so they cannot match unintended files. Methods taking paths
// reject empty path lists because git's own no-pathspec variants either
// no-op successfully (add) or act on the whole work tree (reset, clean).

func pathsGuard(paths []string) error {
	if len(paths) == 0 {
		return &GitError{Code: CodeCommandFailed, Message: "at least one path required", ExitCode: -1}
	}
	return nil
}

func (r *Repo) runPathOp(ctx context.Context, cmd string, flags []string, paths ...string) error {
	args := slices.Concat([]string{cmd}, flags, []string{"--"}, paths)
	_, _, err := runGit(ctx, r.path, args...)
	return err
}

// Stage adds the given paths to the index.
func (r *Repo) Stage(ctx context.Context, paths ...string) error {
	if err := pathsGuard(paths); err != nil {
		return err
	}
	return r.runPathOp(ctx, "add", nil, paths...)
}

// Unstage resets the index for the given paths, leaving the work tree
// untouched. `git restore --staged` is used instead of the classic
// `git reset -- <paths>`: it handles never-committed files identically to
// tracked ones (verified on a fresh add) and states its intent in the
// command itself. The project git floor is well above the 2.23 that
// introduced restore.
func (r *Repo) Unstage(ctx context.Context, paths ...string) error {
	if err := pathsGuard(paths); err != nil {
		return err
	}
	return r.runPathOp(ctx, "restore", []string{"--staged"}, paths...)
}

// StageAll stages modifications, additions and deletions across the whole
// work tree (git add -A).
func (r *Repo) StageAll(ctx context.Context) error {
	return r.runPathOp(ctx, "add", []string{"-A"})
}

// UnstageAll resets the whole index back to HEAD (git reset), leaving the
// work tree untouched.
func (r *Repo) UnstageAll(ctx context.Context) error {
	return r.runPathOp(ctx, "reset", nil)
}

// DiscardChanges reverts work-tree edits for tracked paths and deletes
// untracked ones. Paths are classified via Status first because `git
// restore` refuses untracked files: those go to `git clean -qf`. Status
// collapses a fully-untracked folder to one "dir/" entry, and paths inside
// such an entry are routed to clean as well. Naming a directory (collapsed
// entry or its own path) removes the whole folder, which is the intent
// behind discarding it in a checkbox UI. Ignored and unknown paths fall
// into the restore group, where git fails loudly, so they are never
// cleaned. Restore runs first and aborts the call on failure, so nothing
// is deleted when any requested path is bad. Restore sources from the
// index, so staged changes survive the discard.
func (r *Repo) DiscardChanges(ctx context.Context, paths ...string) error {
	if err := pathsGuard(paths); err != nil {
		return err
	}
	st, err := r.Status(ctx)
	if err != nil {
		return err
	}

	untracked, dirs := map[string]bool{}, []string{}
	for _, f := range st.Files {
		if !f.Untracked {
			continue
		}
		if strings.HasSuffix(f.Path, "/") {
			dirs = append(dirs, f.Path)
		} else {
			untracked[f.Path] = true
		}
	}
	var restore, clean []string
	for _, p := range paths {
		if untracked[p] || slices.ContainsFunc(dirs, func(d string) bool { return strings.HasPrefix(p, d) }) {
			clean = append(clean, p)
		} else {
			restore = append(restore, p)
		}
	}

	if len(restore) > 0 {
		err = r.runPathOp(ctx, "restore", []string{"--worktree"}, restore...)
	}
	if err == nil && len(clean) > 0 {
		err = r.runPathOp(ctx, "clean", []string{"-qf"}, clean...)
	}
	return err
}
