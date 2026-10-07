package git

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"time"
)

// RefKind classifies a commit decoration as git renders it with
// --decorate=short.
type RefKind string

const (
	RefHead   RefKind = "head"   // HEAD points here; Name is the branch or "HEAD" when detached
	RefBranch RefKind = "branch" // local branch at this commit
	RefRemote RefKind = "remote" // remote-tracking ref (name contains "/")
	RefTag    RefKind = "tag"    // annotated or lightweight tag
)

// Ref is one decoration entry attached to a commit.
type Ref struct {
	Kind RefKind `json:"kind"`
	Name string  `json:"name"`
}

// Commit is one entry from `git log --format=` output (issue #23).
// ParentHashes holds zero entries for a root commit, one for a normal
// commit, two or more for merges. Body is the message below the subject
// line, already unwrapped by git's %b.
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

// parseRefs decodes a %D decoration string. git joins entries with ", "
// and ref names never contain spaces, so a bare comma inside an entry is
// part of the name.
func parseRefs(decorations string) []Ref {
	refs := []Ref{}
	if decorations == "" {
		return refs
	}
	for _, item := range strings.Split(decorations, ", ") {
		switch {
		case strings.HasPrefix(item, "tag: "):
			refs = append(refs, Ref{Kind: RefTag, Name: item[len("tag: "):]})
		case strings.HasPrefix(item, "HEAD -> "):
			refs = append(refs, Ref{Kind: RefHead, Name: item[len("HEAD -> "):]})
		case item == "HEAD":
			refs = append(refs, Ref{Kind: RefHead, Name: "HEAD"})
		case strings.Contains(item, "/"):
			refs = append(refs, Ref{Kind: RefRemote, Name: item})
		default:
			refs = append(refs, Ref{Kind: RefBranch, Name: item})
		}
	}
	return refs
}

// logFormat frames each commit with \x1e at the record head and \x1f
// between fields, the conventional gitk/tig machine layout: commit
// messages may contain newlines - even raw \x1f bytes - and must never be
// split on them. A \x1e byte committed into a message would break the
// framing; no % format can survive it and the accepted convention is to
// treat it as out of scope.
const logFormat = "%x1e%H\x1f%h\x1f%P\x1f%an\x1f%ae\x1f%at\x1f%ct\x1f%s\x1f%b\x1f%D"

// LogOptions bounds the listing window. MaxCount and Skip map to
// git log -n/--skip for virtualized "load more"; zero means "no limit".
type LogOptions struct {
	MaxCount int `json:"maxCount"`
	Skip     int `json:"skip"`
}

// Log lists commits newest-first from the current branch. An unborn HEAD
// yields an empty list rather than git's fatal.
func (r *Repo) Log(ctx context.Context, opts LogOptions) ([]Commit, error) {
	if opts.MaxCount < 0 || opts.Skip < 0 {
		return nil, &GitError{Code: CodeValidationFailed, Message: "log window cannot be negative", ExitCode: -1}
	}
	_, _, err := runGit(ctx, r.path, "rev-parse", "--verify", "--quiet", "HEAD")
	if err != nil {
		var ge *GitError
		if errors.As(err, &ge) && ge.Code == CodeCommandFailed && ge.ExitCode == 1 {
			return []Commit{}, nil
		}
		return nil, err
	}
	args := []string{"log", "--decorate=short", "--format=" + logFormat}
	if opts.MaxCount > 0 {
		args = append(args, "-n", strconv.Itoa(opts.MaxCount))
	}
	if opts.Skip > 0 {
		args = append(args, "--skip", strconv.Itoa(opts.Skip))
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
	fields := strings.SplitN(record, "\x1f", 9)
	if len(fields) != 9 {
		return Commit{}, parseFailed("malformed log record: " + clip(record))
	}
	// body may carry \x1f bytes, decorations never can, so the last
	// separator marks the body/decoration boundary
	i := strings.LastIndexByte(fields[8], '\x1f')
	if i < 0 {
		return Commit{}, parseFailed("log record without decoration boundary: " + clip(record))
	}
	body := strings.TrimSuffix(fields[8][:i], "\n")
	decoration := fields[8][i+1:]

	authorSec, err := strconv.ParseInt(fields[5], 10, 64)
	if err != nil {
		return Commit{}, parseFailed("malformed author epoch in record for " + fields[0])
	}
	commitSec, err := strconv.ParseInt(fields[6], 10, 64)
	if err != nil {
		return Commit{}, parseFailed("malformed committer epoch in record for " + fields[0])
	}

	// strings.Fields already yields an empty non-nil slice for a root
	// commit, so %P can be used directly
	parents := strings.Fields(fields[2])
	return Commit{
		Hash:          fields[0],
		ShortHash:     fields[1],
		ParentHashes:  parents,
		AuthorName:    fields[3],
		AuthorEmail:   fields[4],
		AuthorDate:    time.Unix(authorSec, 0).UTC(),
		CommitterDate: time.Unix(commitSec, 0).UTC(),
		Subject:       fields[7],
		Body:          body,
		Refs:          parseRefs(decoration),
	}, nil
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
