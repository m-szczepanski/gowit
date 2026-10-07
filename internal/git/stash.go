package git

import (
	"context"
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
