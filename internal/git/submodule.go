package git

import (
	"context"
	"strings"
)

// SubmoduleState maps the single-column flag of `git submodule status`:
// clean, pointer differs from the recorded one, uninitialized, unmerged,
// or type change (gitlink became a tree/dir or vice versa).
type SubmoduleState string

const (
	SubmoduleOK            SubmoduleState = "ok"
	SubmoduleModified      SubmoduleState = "modified"
	SubmoduleUninitialized SubmoduleState = "uninitialized"
	SubmoduleConflict      SubmoduleState = "conflict"
	SubmoduleTypeChange    SubmoduleState = "type-change"
)

// Submodule is one registered gitlink. SHA is the checked-out commit;
// Describe is git's best label for it (branch/tag/ref), possibly empty.
type Submodule struct {
	Path     string         `json:"path"`
	SHA      string         `json:"sha"`
	Describe string         `json:"describe"`
	State    SubmoduleState `json:"state"`
}

var submoduleFlags = map[byte]SubmoduleState{
	' ': SubmoduleOK,
	'+': SubmoduleModified,
	'-': SubmoduleUninitialized,
	'U': SubmoduleConflict,
	'T': SubmoduleTypeChange,
}

// Submodules lists registered submodules with their state. It parses the
// plain `git submodule status` output; git has no --porcelain for it
// despite the ticket's §3 reference, so the fixed-width sha and the
// single leading flag column are the parse anchors.
func (r *Repo) Submodules(ctx context.Context) ([]Submodule, error) {
	out, _, err := runGit(ctx, r.path, "submodule", "status")
	if err != nil {
		return nil, err
	}
	return parseSubmoduleStatus(string(out))
}

func parseSubmoduleStatus(out string) ([]Submodule, error) {
	var subs []Submodule
	for _, line := range strings.Split(out, "\n") {
		if line == "" {
			continue
		}
		if len(line) < 43 {
			return nil, parseFailed("short submodule status line: " + line)
		}
		state, ok := submoduleFlags[line[0]]
		if !ok {
			return nil, parseFailed("unknown submodule status flag: " + line[:1])
		}
		sha := line[1:41]
		if !isHex(sha) {
			return nil, parseFailed("non-hex submodule sha in: " + line)
		}
		rest := line[42:]
		path, describe, err := splitSubPathDescribe(rest, line)
		if err != nil {
			return nil, err
		}
		subs = append(subs, Submodule{Path: path, SHA: sha, Describe: describe, State: state})
	}
	return subs, nil
}

func splitSubPathDescribe(rest, line string) (string, string, error) {
	// describe is bracketed only when it exists: uninitialized and
	// conflict lines end with the bare path; the caller's length guard
	// already rules out an empty rest
	if rest[len(rest)-1] != ')' {
		return rest, "", nil
	}
	open := strings.LastIndex(rest, " (")
	if open < 0 {
		return "", "", parseFailed("submodule describe bracket without separator: " + line)
	}
	return rest[:open], rest[open+2 : len(rest)-1], nil
}

func isHex(s string) bool {
	for i := 0; i < len(s); i++ {
		c := s[i]
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') && (c < 'A' || c > 'F') {
			return false
		}
	}
	return len(s) > 0
}
