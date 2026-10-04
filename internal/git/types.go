package git

// FileStatus is one working-tree entry, parsed from
// `git status --porcelain=v2`. Status codes follow git's XY grammar
// (index + worktree); see the status task (#15) for the mapping.
type FileStatus struct {
	Path   string
	Status string
}

// Commit is one entry from `git log --format=...`. Extended by the
// history/graph tasks with parents, refs and authorship.
type Commit struct {
	Hash    string
	Subject string
}
