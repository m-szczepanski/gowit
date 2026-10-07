package git

import (
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
