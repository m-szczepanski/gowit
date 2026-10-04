package git

// FileStatus is one working-tree entry, parsed from
// `git status --porcelain=v2`. Status follows git's XY index+worktree grammar.
type FileStatus struct {
	Path   string
	Status string
}

// Commit is one entry from `git log --format=...`.
type Commit struct {
	Hash    string
	Subject string
}
