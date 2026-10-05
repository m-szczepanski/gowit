package git

// FileStatus is one working-tree entry, parsed from
// `git status --porcelain=v2`. XY holds git's index+worktree code pair.
type FileStatus struct {
	XY        string
	Path      string
	OrigPath  string // rename/copy source, empty otherwise
	Submodule string // submodule state field (N..., M..., C...); empty for untracked/ignored
	Untracked bool
	Ignored   bool
	Conflict  bool
}

// Change is a human-readable classification of an entry derived from its
// XY codes and record type.
type Change string

const (
	ChangeAdded       Change = "added"
	ChangeModified    Change = "modified"
	ChangeDeleted     Change = "deleted"
	ChangeRenamed     Change = "renamed"
	ChangeCopied      Change = "copied"
	ChangeTypeChanged Change = "type-changed"
	ChangeConflicted  Change = "conflicted"
	ChangeUntracked   Change = "untracked"
	ChangeIgnored     Change = "ignored"
)

// Change classifies the entry: the staged code wins over the worktree one,
// mirroring how the staging view groups files.
func (f FileStatus) Change() Change {
	if f.Conflict {
		return ChangeConflicted
	}
	if f.Ignored {
		return ChangeIgnored
	}
	if f.Untracked {
		return ChangeUntracked
	}
	code := f.XY[0]
	if code == '.' {
		code = f.XY[1]
	}
	switch code {
	case 'A':
		return ChangeAdded
	case 'D':
		return ChangeDeleted
	case 'R':
		return ChangeRenamed
	case 'C':
		return ChangeCopied
	case 'T':
		return ChangeTypeChanged
	default:
		return ChangeModified
	}
}

// Staged reports a difference between HEAD and the index.
func (f FileStatus) Staged() bool { return !f.Untracked && !f.Ignored && f.XY[0] != '.' }

// Unstaged reports a difference between the index and the work tree.
func (f FileStatus) Unstaged() bool { return !f.Untracked && !f.Ignored && f.XY[1] != '.' }

// BranchStatus is the `# branch.*` header block of git status.
type BranchStatus struct {
	Head     string // branch name; empty when detached
	Oid      string
	Detached bool
	Upstream string // remote tracking ref, empty without one
	Ahead    int
	Behind   int
}

// StatusResult is the full parse of `git status --porcelain=v2 --branch -z`.
type StatusResult struct {
	Branch BranchStatus
	Files  []FileStatus
}

// Commit is one entry from `git log --format=...` (issue #15).
type Commit struct {
	Hash    string
	Subject string
}
