package git

import (
	"context"
	"slices"
	"strconv"
)

// CherryPickOptions selects cherry-pick variants. RecordOriginal adds -x,
// the provenance trailer git GUIs rely on. Mainline chooses the parent for
// merge commits (git's -m); zero means "not set" and a merge ref then
// fails with git's own refusal rather than guessing a parent.
type CherryPickOptions struct {
	RecordOriginal bool `json:"recordOriginal"`
	Mainline       int  `json:"mainline"`
}

// CherryPick applies the given commits or revision ranges, in git's own
// oldest-first order. It moves HEAD, which the watcher already tracks, so
// status, branch and log refreshes ride the #17 flow.
func (r *Repo) CherryPick(ctx context.Context, refs []string, opts CherryPickOptions) error {
	if len(refs) == 0 {
		return &GitError{Code: CodeValidationFailed, Message: "at least one commit ref required", ExitCode: -1}
	}
	for _, ref := range refs {
		if err := guardOptionLike(ref, "commit ref"); err != nil {
			return err
		}
	}
	if opts.Mainline < 0 {
		return &GitError{Code: CodeValidationFailed, Message: "mainline parent cannot be negative", ExitCode: -1}
	}
	args := []string{"cherry-pick"}
	if opts.RecordOriginal {
		args = append(args, "-x")
	}
	if opts.Mainline > 0 {
		args = append(args, "-m", strconv.Itoa(opts.Mainline))
	}
	args = slices.Concat(args, refs)
	_, _, err := runGit(ctx, r.path, args...)
	return err
}
