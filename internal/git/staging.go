package git

import (
	"context"
)

// Paths travel as argv after a literal "--": no shell and no glob
// expansion, so they cannot match unintended files. Methods taking paths
// reject empty path lists because git's own no-pathspec variants either
// no-op successfully (add) or act on the whole work tree (reset, clean).

func (r *Repo) pathsGuard(paths []string) error {
	if len(paths) == 0 {
		return &GitError{Code: CodeCommandFailed, Message: "at least one path required", ExitCode: -1}
	}
	return nil
}

func (r *Repo) runPathOp(ctx context.Context, cmd string, flags []string, paths ...string) error {
	args := append(append([]string{cmd}, flags...), append([]string{"--"}, paths...)...)
	_, _, err := runGit(ctx, r.path, args...)
	return err
}

// Stage adds the given paths to the index.
func (r *Repo) Stage(ctx context.Context, paths ...string) error {
	if err := r.pathsGuard(paths); err != nil {
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
	if err := r.pathsGuard(paths); err != nil {
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
// untracked ones. Paths are classified via Status first: `git restore`
// refuses untracked files, and handing untracked paths to a blanket clean
// would delete directories nobody selected. Ignored and unknown paths fall
// into the restore group, where git fails loudly, so they are never
// cleaned.
func (r *Repo) DiscardChanges(ctx context.Context, paths ...string) error {
	if err := r.pathsGuard(paths); err != nil {
		return err
	}
	st, err := r.Status(ctx)
	if err != nil {
		return err
	}

	untracked := map[string]bool{}
	for _, f := range st.Files {
		if f.Untracked {
			untracked[f.Path] = true
		}
	}
	var restore, clean []string
	for _, p := range paths {
		if untracked[p] {
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
