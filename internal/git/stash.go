package git

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"
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

// StashEntry is one `git stash list` row. Index is the git-visible
// position (stash@{n}); Type distinguishes a named push ("message",
// reflog subject "On <branch>: ...") from a bare WIP ("wip"), with
// "other" for anything else. Date is the stash commit's committer time, UTC.
type StashEntry struct {
	Index   int       `json:"index"`
	Message string    `json:"message"`
	Type    string    `json:"type"`
	Date    time.Time `json:"date"`
}

// stash list is a log-family command: %x00 is the NUL escape there,
// while for-each-ref needs %00.
const stashListFormat = "%gd%x00%gs%x00%ct"

// StashList returns the entries newest first, the order git maintains.
func (r *Repo) StashList(ctx context.Context) ([]StashEntry, error) {
	out, _, err := runGit(ctx, r.path, "stash", "list", "--format="+stashListFormat)
	if err != nil {
		return nil, err
	}
	return parseStashList(string(out))
}

func parseStashList(out string) ([]StashEntry, error) {
	var entries []StashEntry
	for _, line := range strings.Split(out, "\n") {
		if line == "" {
			continue
		}
		fields := strings.Split(line, "\x00")
		if len(fields) != 3 {
			return nil, parseFailed("stash record wants 3 fields, got " + strconv.Itoa(len(fields)))
		}
		idx, err := parseStashSelector(fields[0])
		if err != nil {
			return nil, err
		}
		sec, err := strconv.ParseInt(fields[2], 10, 64)
		if err != nil {
			return nil, parseFailed(`bad stash committerdate "` + fields[2] + `"`)
		}
		entries = append(entries, StashEntry{
			Index:   idx,
			Message: fields[1],
			Type:    stashType(fields[1]),
			Date:    time.Unix(sec, 0).UTC(),
		})
	}
	return entries, nil
}

func parseStashSelector(sel string) (int, error) {
	rest, ok := strings.CutPrefix(sel, "stash@{")
	if !ok {
		return 0, parseFailed("unexpected stash selector: " + sel)
	}
	num := strings.TrimSuffix(rest, "}")
	idx, err := strconv.Atoi(num)
	if err != nil || idx < 0 {
		return 0, parseFailed("unexpected stash selector: " + sel)
	}
	return idx, nil
}

func stashType(subject string) string {
	switch {
	case strings.HasPrefix(subject, "WIP on "):
		return "wip"
	case strings.HasPrefix(subject, "On "):
		return "message"
	default:
		return "other"
	}
}

// StashApply restores a stash entry and keeps the entry, so a failed
// restore loses nothing. Index is the position from StashList. Applying
// rewrites work-tree files, which the watcher (#17) turns into a status
// refresh; stash ops never move HEAD, so branch and log caches stay valid
// untouched.
func (r *Repo) StashApply(ctx context.Context, idx int) error {
	ref, err := stashRef(idx)
	if err != nil {
		return err
	}
	_, _, err = runGit(ctx, r.path, "stash", "apply", ref)
	return stashConflict(err)
}

// StashPop restores and drops the entry. git refuses the drop while the
// restore conflicts, so the entry survives exactly like StashApply.
func (r *Repo) StashPop(ctx context.Context, idx int) error {
	ref, err := stashRef(idx)
	if err != nil {
		return err
	}
	_, _, err = runGit(ctx, r.path, "stash", "pop", ref)
	return stashConflict(err)
}

// StashDrop removes the entry without touching the work tree.
func (r *Repo) StashDrop(ctx context.Context, idx int) error {
	ref, err := stashRef(idx)
	if err != nil {
		return err
	}
	_, _, err = runGit(ctx, r.path, "stash", "drop", ref)
	return err
}

func stashRef(idx int) (string, error) {
	if idx < 0 {
		return "", &GitError{Code: CodeValidationFailed, Message: "stash index cannot be negative", ExitCode: -1}
	}
	return fmt.Sprintf("stash@{%d}", idx), nil
}

// stashConflict upgrades the merge-conflict shape (apply/pop printing
// CONFLICT) to ErrStashConflict; the untracked-collision failure already
// arrives classified from classify.
func stashConflict(err error) error {
	var ge *GitError
	if errors.As(err, &ge) && ge.Code == CodeConflict {
		return &GitError{Code: CodeStashConflict, Message: ge.Message, ExitCode: ge.ExitCode}
	}
	return err
}

// StashClear deletes every stash entry. Destructive without a git-side
// undo (the commits linger only until gc prunes them), so the UI must
// confirm before calling.
func (r *Repo) StashClear(ctx context.Context) error {
	_, _, err := runGit(ctx, r.path, "stash", "clear")
	return err
}
