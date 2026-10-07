package git

import (
	"context"
	"slices"
)

// StashPushOptions selects the `git stash push` variants. Paths scope the
// stash to a pathspec (git's default "all" applies when empty);
// IncludeUntracked also stashes untracked files; Message names the entry.
type StashPushOptions struct {
	Message          string   `json:"message"`
	IncludeUntracked bool     `json:"includeUntracked"`
	Paths            []string `json:"paths,omitempty"`
}

// StashPush records the current work in a new stash entry. A clean tree is
// not an error: git no-ops with "No local changes to save" and creates no
// entry, and that outcome passes through unchanged.
func (r *Repo) StashPush(ctx context.Context, opts StashPushOptions) error {
	args := []string{"stash", "push"}
	if opts.IncludeUntracked {
		args = append(args, "-u")
	}
	if opts.Message != "" {
		args = append(args, "-m", opts.Message)
	}
	if len(opts.Paths) > 0 {
		args = slices.Concat(args, []string{"--"}, opts.Paths)
	}
	_, _, err := runGit(ctx, r.path, args...)
	return err
}
