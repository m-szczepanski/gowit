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
