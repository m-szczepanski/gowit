package git

import (
	"context"
	"errors"
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
	return cherryPickErr(err)
}

// CherryPickState reports a stopped or running sequence: the commit being
// picked (git's CHERRY_PICK_HEAD) and the unmerged paths, read from the
// same porcelain v2 parse that Status uses.
type CherryPickState struct {
	InProgress    bool     `json:"inProgress"`
	Head          string   `json:"head,omitempty"`
	ConflictPaths []string `json:"conflictPaths,omitempty"`
}

func (r *Repo) CherryPickState(ctx context.Context) (*CherryPickState, error) {
	out, _, err := runGit(ctx, r.path, "rev-parse", "-q", "--verify", "CHERRY_PICK_HEAD")
	if err != nil {
		if ge, ok := err.(*GitError); ok && ge.ExitCode == 1 {
			return &CherryPickState{}, nil
		}
		return nil, err
	}
	res, err := r.Status(ctx)
	if err != nil {
		return nil, err
	}
	st := &CherryPickState{InProgress: true, Head: firstLine(string(out))}
	for _, f := range res.Files {
		if f.Conflict {
			st.ConflictPaths = append(st.ConflictPaths, f.Path)
		}
	}
	return st, nil
}

// CherryPickContinue commits the resolved conflicts and carries on with
// the rest of the sequence.
func (r *Repo) CherryPickContinue(ctx context.Context) error {
	_, _, err := runGit(ctx, r.path, "cherry-pick", "--continue")
	return cherryPickErr(err)
}

// CherryPickAbort cancels the sequence and returns to its starting point.
func (r *Repo) CherryPickAbort(ctx context.Context) error {
	_, _, err := runGit(ctx, r.path, "cherry-pick", "--abort")
	return err
}

// CherryPickSkip drops the current commit and proceeds with the rest.
func (r *Repo) CherryPickSkip(ctx context.Context) error {
	_, _, err := runGit(ctx, r.path, "cherry-pick", "--skip")
	return cherryPickErr(err)
}

// cherryPickErr specializes the generic merge conflict for the cherry-pick
// sequencer; the empty-commit stop already arrives typed from classify,
// and git's "would overwrite local changes" refusal stays command_failed
// because it touches nothing, so it is a refusal, not a conflict.
func cherryPickErr(err error) error {
	var ge *GitError
	if errors.As(err, &ge) && ge.Code == CodeConflict {
		return &GitError{Code: CodeCherryPickConflict, Message: ge.Message, ExitCode: ge.ExitCode}
	}
	return err
}
