package git

import (
	"context"
	"strconv"
	"strings"
	"time"
)

// BranchScope selects which ref namespaces Branches walks.
type BranchScope int

const (
	BranchScopeLocal BranchScope = iota
	BranchScopeRemote
	BranchScopeAll
)

// Branch is one entry of the branch list. Name is display-level: a bare
// branch name for locals, and remote-qualified (origin/main) for remotes.
// Ahead/Behind compare against Upstream; both are 0 when there is no
// upstream or it has gone. CommitterDate is UTC.
type Branch struct {
	Name          string    `json:"name"`
	Ref           string    `json:"ref"`
	IsLocal       bool      `json:"isLocal"`
	IsRemote      bool      `json:"isRemote"`
	IsCurrent     bool      `json:"isCurrent"`
	Head          string    `json:"head"`
	Upstream      string    `json:"upstream,omitempty"`
	Ahead         int       `json:"ahead"`
	Behind        int       `json:"behind"`
	Subject       string    `json:"subject"`
	CommitterDate time.Time `json:"committerDate"`
}

const branchFormat = "%(refname)%00%(objectname)%00%(upstream:short)%00%(upstream:track)%00%(subject)%00%(committerdate:unix)"

// Branches lists refs in the chosen scope, sorted by full ref name the way
// for-each-ref sorts, and marks the checked-out local branch as current.
// Remote HEAD aliases (origin/HEAD) are left out.
func (r *Repo) Branches(ctx context.Context, scope BranchScope) ([]Branch, error) {
	args := []string{"for-each-ref", "--format=" + branchFormat,
		// origin/HEAD is a symref alias for the default branch, not a
		// branch; listing it as one duplicates origin/main.
		"--exclude=refs/remotes/*/HEAD",
	}
	switch scope {
	case BranchScopeLocal:
		args = append(args, "refs/heads")
	case BranchScopeRemote:
		args = append(args, "refs/remotes")
	case BranchScopeAll:
		args = append(args, "refs/heads", "refs/remotes")
	default:
		return nil, &GitError{Code: CodeValidationFailed, Message: "unknown branch scope", ExitCode: -1}
	}
	out, _, err := runGit(ctx, r.path, args...)
	if err != nil {
		return nil, err
	}
	branches, err := parseBranchRecords(string(out))
	if err != nil {
		return nil, err
	}
	name, err := r.currentBranch(ctx)
	if err != nil {
		return nil, err
	}
	for i := range branches {
		branches[i].IsCurrent = branches[i].IsLocal && branches[i].Name == name
	}
	return branches, nil
}

// currentBranch returns the checked-out branch's short name, or "" when
// HEAD is detached. symbolic-ref exits 1 exactly for the detached case.
func (r *Repo) currentBranch(ctx context.Context) (string, error) {
	out, _, err := runGit(ctx, r.path, "symbolic-ref", "-q", "--short", "HEAD")
	if err != nil {
		if ge, ok := err.(*GitError); ok && ge.ExitCode == 1 {
			return "", nil
		}
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func parseBranchRecords(out string) ([]Branch, error) {
	var branches []Branch
	for _, line := range strings.Split(out, "\n") {
		if line == "" {
			continue
		}
		fields := strings.Split(line, "\x00")
		if len(fields) != 6 {
			return nil, parseFailed("branch record wants 6 fields, got " + strconv.Itoa(len(fields)))
		}
		ref, oid, upstream, track, subject, dateSec := fields[0], fields[1], fields[2], fields[3], fields[4], fields[5]
		if oid == "" {
			return nil, parseFailed("branch record without object id: " + ref)
		}
		ahead, behind, err := parseUpstreamTrack(track)
		if err != nil {
			return nil, err
		}
		sec, err := strconv.ParseInt(dateSec, 10, 64)
		if err != nil {
			return nil, parseFailed(`bad committerdate "` + dateSec + `" for ` + ref)
		}
		b := Branch{
			Ref:           ref,
			Head:          oid,
			Upstream:      upstream,
			Ahead:         ahead,
			Behind:        behind,
			Subject:       subject,
			CommitterDate: time.Unix(sec, 0).UTC(),
		}
		switch {
		case strings.HasPrefix(ref, "refs/heads/"):
			b.IsLocal = true
			b.Name = strings.TrimPrefix(ref, "refs/heads/")
		case strings.HasPrefix(ref, "refs/remotes/"):
			b.IsRemote = true
			b.Name = strings.TrimPrefix(ref, "refs/remotes/")
		default:
			return nil, parseFailed("unexpected ref namespace: " + ref)
		}
		branches = append(branches, b)
	}
	return branches, nil
}

// parseUpstreamTrack decodes for-each-ref's %(upstream:track): the empty
// string (no upstream), "[gone]", "[ahead N]", "[behind N]", or the
// diverged pair "[ahead N, behind M]".
func parseUpstreamTrack(track string) (int, int, error) {
	inner := strings.TrimSuffix(strings.TrimPrefix(track, "["), "]")
	if inner == "" || inner == "gone" {
		return 0, 0, nil
	}
	ahead, behind := 0, 0
	for _, part := range strings.Split(inner, ", ") {
		word, num, ok := strings.Cut(part, " ")
		if !ok {
			return 0, 0, parseFailed("malformed upstream track: " + track)
		}
		n, err := strconv.Atoi(num)
		if err != nil {
			return 0, 0, parseFailed("malformed upstream track: " + track)
		}
		switch word {
		case "ahead":
			ahead = n
		case "behind":
			behind = n
		default:
			return 0, 0, parseFailed("malformed upstream track: " + track)
		}
	}
	return ahead, behind, nil
}
