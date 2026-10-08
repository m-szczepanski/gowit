package git

import (
	"context"
	"slices"
)

// CherryPickOptions selects cherry-pick variants. RecordOriginal adds -x,
// the provenance trailer git GUIs rely on.
type CherryPickOptions struct {
	RecordOriginal bool `json:"recordOriginal"`
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
	args := []string{"cherry-pick"}
	if opts.RecordOriginal {
		args = append(args, "-x")
	}
	args = slices.Concat(args, refs)
	_, _, err := runGit(ctx, r.path, args...)
	return err
}
