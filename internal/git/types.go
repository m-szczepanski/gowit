package git

// FileStatus is one working-tree entry, parsed from
// `git status --porcelain=v2`. XY holds git's index+worktree code pair.
// Staged, Unstaged and Change are derived by classify during parsing so
// the serialized payload carries the full picture: the frontend must never
// re-derive git's XY grammar.
type FileStatus struct {
	XY        string       `json:"xy"`
	Path      string       `json:"path"`
	OrigPath  string       `json:"origPath,omitempty"`
	Submodule string       `json:"submodule,omitempty"`
	Untracked bool         `json:"untracked"`
	Ignored   bool         `json:"ignored"`
	Conflict  bool         `json:"conflict"`
	Staged    bool         `json:"staged"`
	Unstaged  bool         `json:"unstaged"`
	Change    Change       `json:"change"`
	Stages    []MergeStage `json:"stages,omitempty"`
}

// MergeStage is one index stage of a conflicted path, positional in the
// porcelain v2 unmerged record (stage 1 base, 2 ours, 3 theirs).
type MergeStage struct {
	Stage int    `json:"stage"`
	Mode  string `json:"mode"`
	Oid   string `json:"oid"`
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
	ChangeUnknown     Change = "unknown"
)

// classifyFile fills the derived fields (Staged, Unstaged, Change) from the
// record data. The staged code wins over the worktree one for Change,
// mirroring how the staging view groups files.
func classifyFile(f FileStatus) FileStatus {
	f.Staged = !f.Untracked && !f.Ignored && f.XY[0] != '.'
	f.Unstaged = !f.Untracked && !f.Ignored && f.XY[1] != '.'
	f.Change = changeOf(f)
	return f
}

func changeOf(f FileStatus) Change {
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
	return changeFromLetter(code)
}

// changeFromLetter maps git's single-letter change status - the name-status
// code and the porcelain XY code share it.
func changeFromLetter(code byte) Change {
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
	case 'M':
		return ChangeModified
	default:
		return ChangeUnknown
	}
}

// BranchStatus is the `# branch.*` header block of git status.
type BranchStatus struct {
	Head     string `json:"head"` // branch name; empty when detached
	Oid      string `json:"oid"`
	Detached bool   `json:"detached"`
	Upstream string `json:"upstream,omitempty"`
	Ahead    int    `json:"ahead"`
	Behind   int    `json:"behind"`
}

// StatusResult is the full parse of `git status --porcelain=v2 --branch -z`.
type StatusResult struct {
	Branch BranchStatus
	Files  []FileStatus
}
