package git

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"time"
)

// RefKind classifies a commit decoration as git renders it with
// --decorate=full.
type RefKind string

const (
	RefHead   RefKind = "head"   // HEAD points here; Name is the branch or "HEAD" when detached
	RefBranch RefKind = "branch" // local branch at this commit
	RefRemote RefKind = "remote" // remote-tracking ref (refs/remotes/*)
	RefTag    RefKind = "tag"    // annotated or lightweight tag
)

// Ref is one decoration entry attached to a commit, named without its
// refs/ namespace.
type Ref struct {
	Kind RefKind `json:"kind"`
	Name string  `json:"name"`
}

// Commit is one entry from `git log --format=` output (issue #23).
// ParentHashes holds zero entries for a root commit, one for a normal
// commit, two or more for merges. Body is the message below the subject.
type Commit struct {
	Hash          string    `json:"hash"`
	ShortHash     string    `json:"shortHash"`
	ParentHashes  []string  `json:"parentHashes"`
	AuthorName    string    `json:"authorName"`
	AuthorEmail   string    `json:"authorEmail"`
	AuthorDate    time.Time `json:"authorDate"`
	CommitterDate time.Time `json:"committerDate"`
	Subject       string    `json:"subject"`
	Body          string    `json:"body"`
	Refs          []Ref     `json:"refs"`
}

// parseRefs decodes a %D decoration string produced by --decorate=full:
// entries carry their full refs/heads, refs/tags or refs/remotes
// namespace, which is what keeps a local branch named feature/x from
// being mistaken for a remote. git joins entries with ", " and ref names
// contain no spaces, so a bare comma inside an entry is part of the name.
func parseRefs(decorations string) []Ref {
	refs := []Ref{}
	if decorations == "" {
		return refs
	}
	for _, item := range strings.Split(decorations, ", ") {
		if name, ok := strings.CutPrefix(item, "tag: "); ok {
			refs = append(refs, Ref{Kind: RefTag, Name: trimRefNamespace(name)})
		} else if head, ok := strings.CutPrefix(item, "HEAD -> "); ok {
			refs = append(refs, Ref{Kind: RefHead, Name: trimRefNamespace(head)})
		} else {
			switch {
			case item == "HEAD":
				refs = append(refs, Ref{Kind: RefHead, Name: "HEAD"})
			case strings.HasPrefix(item, "refs/remotes/"):
				refs = append(refs, Ref{Kind: RefRemote, Name: trimRefNamespace(item)})
			default:
				refs = append(refs, Ref{Kind: RefBranch, Name: trimRefNamespace(item)})
			}
		}
	}
	return refs
}

// trimRefNamespace drops whichever refs/ namespace git attached under
// --decorate=full, keeping the bare name a renderer displays. Unknown
// namespaces (refs/stash, refs/notes) keep their full name.
func trimRefNamespace(name string) string {
	for _, ns := range []string{"refs/heads/", "refs/tags/", "refs/remotes/"} {
		if trimmed, ok := strings.CutPrefix(name, ns); ok {
			return trimmed
		}
	}
	return name
}

// logFormat frames one commit per record: \x1e opens the record, \x1f
// separates fields. Commit messages are raw %B, which may contain
// newlines and even \x1f bytes, so the message and the decoration field
// are split at the LAST separator, the only field ref names cannot
// contain one of. Two residual limits are accepted, same convention as
// gitk/tig: a \x1e byte inside a message breaks record framing, and a
// \x1f byte inside an author name or email shifts fields; both are
// pathological input to config-controlled identity strings.
const logFormat = "%x1e%H\x1f%h\x1f%P\x1f%at\x1f%ct\x1f%an\x1f%ae\x1f%B\x1f%D"

// LogOptions bounds the listing and its history source. MaxCount and Skip
// map to git log -n/--skip for virtualized "load more"; zero means no
// limit. Ref starts the walk at any revision, empty means HEAD. Path
// restricts the listing to commits touching that one repository-relative
// file. FirstParent follows only first parents, the history view a
// merge-based UI shows. All lists every ref. Ref and All make the
// unborn-HEAD probe inapplicable because their starting point is not HEAD.
type LogOptions struct {
	MaxCount    int
	Skip        int
	Ref         string
	Path        string
	FirstParent bool
	All         bool
}

// Log lists commits newest-first. An unborn HEAD yields an empty list
// rather than git's fatal.
func (r *Repo) Log(ctx context.Context, opts LogOptions) ([]Commit, error) {
	if opts.MaxCount < 0 || opts.Skip < 0 {
		return nil, &GitError{Code: CodeValidationFailed, Message: "log window cannot be negative", ExitCode: -1}
	}
	if err := guardOptionLike(opts.Ref, "ref"); err != nil {
		return nil, err
	}
	path := ""
	if opts.Path != "" {
		clean, err := cleanDiffPath(opts.Path)
		if err != nil {
			return nil, err
		}
		path = clean
	}

	// the unborn-HEAD probe only describes the default starting point;
	// explicit refs or --all must reach git itself
	if opts.Ref == "" && !opts.All {
		_, _, err := runGit(ctx, r.path, "rev-parse", "--verify", "--quiet", "HEAD")
		if err != nil {
			var ge *GitError
			if errors.As(err, &ge) && ge.Code == CodeCommandFailed && ge.ExitCode == 1 {
				return []Commit{}, nil
			}
			return nil, err
		}
	}

	args := []string{"log", "--decorate=full", "--format=" + logFormat}
	if opts.FirstParent {
		args = append(args, "--first-parent")
	}
	if opts.All {
		args = append(args, "--all")
	}
	if opts.MaxCount > 0 {
		args = append(args, "-n", strconv.Itoa(opts.MaxCount))
	}
	if opts.Skip > 0 {
		args = append(args, "--skip", strconv.Itoa(opts.Skip))
	}
	if opts.Ref != "" {
		args = append(args, opts.Ref)
	}
	if path != "" {
		args = append(args, "--", ":(literal)"+path)
	}
	out, _, err := runGit(ctx, r.path, args...)
	if err != nil {
		return nil, err
	}
	return parseLogOutput(out)
}

func parseLogOutput(out []byte) ([]Commit, error) {
	commits := []Commit{}
	// records[0] is the text before the first \x1e, empty by construction
	for _, record := range strings.Split(string(out), "\x1e")[1:] {
		commit, err := parseLogRecord(record)
		if err != nil {
			return nil, err
		}
		commits = append(commits, commit)
	}
	return commits, nil
}

func parseLogRecord(record string) (Commit, error) {
	record = strings.TrimSuffix(record, "\n")
	fields := strings.SplitN(record, "\x1f", 8)
	if len(fields) != 8 {
		return Commit{}, parseFailed("malformed log record: " + clip(record))
	}
	i := strings.LastIndexByte(fields[7], '\x1f')
	if i < 0 {
		return Commit{}, parseFailed("log record without decoration boundary: " + clip(record))
	}
	subject, body := deriveMessage(fields[7][:i])
	decoration := fields[7][i+1:]

	authorSec, err := strconv.ParseInt(fields[3], 10, 64)
	if err != nil {
		return Commit{}, parseFailed("malformed author epoch in record for " + fields[0])
	}
	commitSec, err := strconv.ParseInt(fields[4], 10, 64)
	if err != nil {
		return Commit{}, parseFailed("malformed committer epoch in record for " + fields[0])
	}

	return Commit{
		Hash:          fields[0],
		ShortHash:     fields[1],
		ParentHashes:  strings.Fields(fields[2]),
		AuthorName:    fields[5],
		AuthorEmail:   fields[6],
		AuthorDate:    time.Unix(authorSec, 0).UTC(),
		CommitterDate: time.Unix(commitSec, 0).UTC(),
		Subject:       subject,
		Body:          body,
		Refs:          parseRefs(decoration),
	}, nil
}

// deriveMessage splits git's raw %B the way git derives %s and %b: the
// subject is the first paragraph with its lines space-joined (a message
// without a blank separator line is all subject), the body is what
// follows the first blank line, minus the trailing newline every commit
// object carries.
func deriveMessage(message string) (subject, body string) {
	message = strings.TrimSuffix(message, "\n")
	first, rest := message, ""
	if i := strings.Index(message, "\n\n"); i >= 0 {
		first, rest = message[:i], strings.TrimSuffix(message[i+2:], "\n")
	}
	return strings.Join(strings.Split(first, "\n"), " "), rest
}

// clip truncates a record for error text so a megabyte commit message
// cannot flood a GitError.
func clip(s string) string {
	const n = 120
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
