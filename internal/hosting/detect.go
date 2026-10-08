package hosting

import (
	"net/url"
	"strings"
)

// Remote is a classified hosting coordinate parsed from a git remote
// URL. Owner keeps GitLab subgroup nesting ("group/sub") by taking the
// last path segment as the repo; pasted browser URLs with trailing paths
// (/settings) therefore misread and are out of scope. Credentials
// embedded in the URL are dropped, never carried.
type Remote struct {
	Kind  Kind   `json:"kind"`
	Host  string `json:"host"`
	Owner string `json:"owner"`
	Repo  string `json:"repo"`
}

// Detect parses scp-ish (git@host:path), https, and ssh:// remote forms
// and classifies the provider. Host labels github.* or gitlab.* (or their
// exact domains) select the kind; everything else is unknown but still
// parsed into host/owner/repo.
func Detect(raw string) (Remote, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return Remote{}, invalid("remote URL is empty")
	}

	var host, path string
	if u, err := url.Parse(trimmed); err == nil && u.Scheme != "" {
		switch u.Scheme {
		case "https", "http", "ssh", "git":
			host = u.Hostname()
			path = u.Path
		default:
			return Remote{}, invalid("unsupported remote URL scheme: " + trimmed)
		}
	} else {
		h, p, ok := strings.Cut(trimmed, ":")
		if !ok || p == "" || strings.Contains(p, "://") {
			return Remote{}, invalid("unparsable remote URL: " + trimmed)
		}
		if at := strings.LastIndex(h, "@"); at >= 0 {
			h = h[at+1:]
		}
		host = h
		path = p
	}

	segments := strings.Split(strings.Trim(path, "/"), "/")
	for i, s := range segments {
		segments[i] = strings.TrimSuffix(s, ".git")
	}
	if len(segments) < 2 || segments[0] == "" || segments[len(segments)-1] == "" {
		return Remote{}, invalid("remote URL must name owner and repository: " + trimmed)
	}
	if host == "" {
		return Remote{}, invalid("remote URL has no host: " + trimmed)
	}
	host = strings.ToLower(host)
	repo := segments[len(segments)-1]
	owner := strings.Join(segments[:len(segments)-1], "/")
	return Remote{Kind: classify(host), Host: host, Owner: owner, Repo: repo}, nil
}

func classify(host string) Kind {
	if kind, ok := classifyLabel(host, "github", KindGitHub); ok {
		return kind
	}
	if kind, ok := classifyLabel(host, "gitlab", KindGitLab); ok {
		return kind
	}
	return KindUnknown
}

func classifyLabel(host, label string, kind Kind) (Kind, bool) {
	first, last, found := strings.Cut(host, ".")
	if !found {
		return KindUnknown, false
	}
	if first == label || last == label || strings.HasSuffix(host, "."+label+".com") {
		return kind, true
	}
	return KindUnknown, false
}

func invalid(msg string) error { return &Error{Kind: ErrKindInvalid, Message: msg} }
