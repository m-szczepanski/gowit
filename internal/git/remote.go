package git

import (
	"context"
	"strings"
)

// Remote is one configured remote with the URLs git would use for each
// direction. A remote configured only for pushing has an empty FetchURL.
type Remote struct {
	Name     string `json:"name"`
	FetchURL string `json:"fetchUrl,omitempty"`
	PushURL  string `json:"pushUrl,omitempty"`
}

// Remotes lists the configured remotes in git's own order.
func (r *Repo) Remotes(ctx context.Context) ([]Remote, error) {
	out, _, err := runGit(ctx, r.path, "remote", "-v")
	if err != nil {
		return nil, err
	}
	return parseRemoteV(string(out))
}

// parseRemoteV decodes `git remote -v`: one "name<TAB>url (fetch|push)"
// line per URL. Fetch and push share a line pair when the URLs match,
// and a pushurl-only remote prints a bare, informationless line for the
// missing fetch URL. The first non-empty URL per kind wins; extra URLs
// belong to mirror setups outside the app's scope.
func parseRemoteV(out string) ([]Remote, error) {
	list := []Remote{}
	at := map[string]int{}
	for _, line := range strings.Split(out, "\n") {
		if line == "" {
			continue
		}
		name, rest, ok := strings.Cut(line, "\t")
		if !ok || name == "" {
			return nil, parseFailed("malformed remote line: " + line)
		}
		// a remote without a fetch url prints that line bare: name and
		// tab, no url and no "(fetch)" suffix
		if strings.TrimSpace(rest) == "" {
			continue
		}
		kind := "fetch"
		url, trimmed := strings.CutSuffix(rest, " (fetch)")
		if !trimmed {
			if url, trimmed = strings.CutSuffix(rest, " (push)"); !trimmed {
				return nil, parseFailed("malformed remote line: " + line)
			}
			kind = "push"
		}
		if url == "" {
			continue
		}
		i, seen := at[name]
		if !seen {
			at[name] = len(list)
			list = append(list, Remote{Name: name})
			i = len(list) - 1
		}
		if kind == "fetch" && list[i].FetchURL == "" {
			list[i].FetchURL = url
		} else if kind == "push" && list[i].PushURL == "" {
			list[i].PushURL = url
		}
	}
	return list, nil
}

// Upstream is the remote branch a local branch tracks. Remote and Branch
// split Ref at the first slash; a tracked local ref (no remote) leaves
// Remote empty.
type Upstream struct {
	Ref    string `json:"ref"`
	Remote string `json:"remote"`
	Branch string `json:"branch"`
}

// Upstream resolves the tracking ref configured for branch via
// `git rev-parse --abbrev-ref <branch>@{upstream}`. A branch without an
// upstream surfaces the typed ErrNoUpstream; an unknown branch surfaces
// command_failed so callers can tell "not configured" from "no such
// branch".
func (r *Repo) Upstream(ctx context.Context, branch string) (Upstream, error) {
	if branch == "" {
		return Upstream{}, &GitError{Code: CodeValidationFailed, Message: "branch required", ExitCode: -1}
	}
	if err := guardOptionLike(branch, "branch"); err != nil {
		return Upstream{}, err
	}
	out, _, err := runGit(ctx, r.path, "rev-parse", "--abbrev-ref", branch+"@{upstream}")
	if err != nil {
		return Upstream{}, err
	}
	ref := strings.TrimSpace(string(out))
	up := Upstream{Ref: ref, Branch: ref}
	if remote, local, found := strings.Cut(ref, "/"); found {
		up.Remote, up.Branch = remote, local
	}
	return up, nil
}

// CapabilityIssue names why remote operations are unavailable, so the
// toolbar can explain a disabled state instead of only greying it out.
type CapabilityIssue string

const (
	IssueDetachedHead CapabilityIssue = "detached_head"
	IssueNoUpstream   CapabilityIssue = "no_upstream"
)

// Capabilities is the toolbar state for the current branch: the
// porcelain v2 branch block already resolves detached HEAD and the
// upstream of the checked-out branch, so no extra plumbing is needed.
type Capabilities struct {
	Detached bool            `json:"detached"`
	Upstream string          `json:"upstream,omitempty"`
	CanPush  bool            `json:"canPush"`
	CanPull  bool            `json:"canPull"`
	Reason   CapabilityIssue `json:"reason,omitempty"`
}

// Capabilities reports whether push/pull make sense right now. A branch
// with no upstream reports IssueNoUpstream so the UI can offer
// --set-upstream; a detached HEAD reports IssueDetachedHead.
func (r *Repo) Capabilities(ctx context.Context) (*Capabilities, error) {
	st, err := r.Status(ctx)
	if err != nil {
		return nil, err
	}
	c := &Capabilities{Detached: st.Branch.Detached, Upstream: st.Branch.Upstream}
	switch {
	case c.Detached:
		c.Reason = IssueDetachedHead
	case c.Upstream == "":
		c.Reason = IssueNoUpstream
	default:
		c.CanPush, c.CanPull = true, true
	}
	return c, nil
}

// SetUpstream points branch at remote/remoteBranch, the explicit form
// of the --set-upstream offer the disabled-toolbar state surfaces. The
// upstream ref must already exist locally (fetched or pushed); git
// refuses otherwise and the failure surfaces as command_failed.
func (r *Repo) SetUpstream(ctx context.Context, branch, remote, remoteBranch string) error {
	for _, v := range []struct {
		label, value string
	}{
		{"branch", branch},
		{"remote", remote},
		{"remote branch", remoteBranch},
	} {
		if v.value == "" {
			return &GitError{Code: CodeValidationFailed, Message: v.label + " required", ExitCode: -1}
		}
		if err := guardOptionLike(v.value, v.label); err != nil {
			return err
		}
	}
	_, _, err := runGit(ctx, r.path, "branch", "--set-upstream-to="+remote+"/"+remoteBranch, branch)
	return err
}
