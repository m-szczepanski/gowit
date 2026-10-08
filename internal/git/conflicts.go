package git

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
)

// Conflict is one unmerged path. A nil stage means that side has no
// entry: add/add has no Base, a side that deleted the file has no Ours
// or Theirs. XY is git's raw porcelain code (UU, AA, DU, UD, ...) which
// tells the two delete flavors apart.
type Conflict struct {
	Path   string      `json:"path"`
	XY     string      `json:"xy"`
	Base   *MergeStage `json:"base,omitempty"`
	Ours   *MergeStage `json:"ours,omitempty"`
	Theirs *MergeStage `json:"theirs,omitempty"`
}

// zeroOID marks an index stage without a side; status.go keeps it in the
// parsed record, here it becomes the absent pointer.
const zeroOID = "0000000000000000000000000000000000000000"

// Conflicts enumerates unmerged paths from the same porcelain v2 u
// records that Status parses (#14), so the conflict view and the staging
// view can never disagree about which files are conflicted.
func (r *Repo) Conflicts(ctx context.Context) ([]Conflict, error) {
	res, err := r.Status(ctx)
	if err != nil {
		return nil, err
	}
	var conflicts []Conflict
	for _, f := range res.Files {
		if !f.Conflict {
			continue
		}
		c := Conflict{Path: f.Path, XY: f.XY}
		stages := map[int]MergeStage{}
		for _, s := range f.Stages {
			if s.Oid != zeroOID {
				stages[s.Stage] = s
			}
		}
		if s, ok := stages[1]; ok {
			c.Base = &s
		}
		if s, ok := stages[2]; ok {
			c.Ours = &s
		}
		if s, ok := stages[3]; ok {
			c.Theirs = &s
		}
		conflicts = append(conflicts, c)
	}
	return conflicts, nil
}

// ConflictContent carries the 3-way data for one conflicted path. Stages
// is keyed by git's stage number (1 base, 2 ours, 3 theirs) and holds an
// entry only for sides that exist: add/add has no 1, delete shapes have
// no 2 or 3. Working holds the merged work-tree bytes (conflict markers
// included); WorkingExists distinguishes an absent file from an empty one.
type ConflictContent struct {
	Stages        map[int][]byte `json:"stages"`
	Working       []byte         `json:"working,omitempty"`
	WorkingExists bool           `json:"workingExists"`
}

// ConflictStages reads each side through git show :N:path, so contents
// come from the index exactly as git resolved them into stages.
func (r *Repo) ConflictStages(ctx context.Context, path string) (*ConflictContent, error) {
	clean, err := cleanRepoPath(path)
	if err != nil {
		return nil, err
	}
	conflicts, err := r.Conflicts(ctx)
	if err != nil {
		return nil, err
	}
	var found *Conflict
	for i := range conflicts {
		if conflicts[i].Path == clean {
			found = &conflicts[i]
			break
		}
	}
	if found == nil {
		return nil, &GitError{Code: CodeValidationFailed, Message: "path is not conflicted: " + clean, ExitCode: -1}
	}
	cc := &ConflictContent{Stages: map[int][]byte{}}
	sides := []struct {
		stage int
		rec   *MergeStage
	}{{1, found.Base}, {2, found.Ours}, {3, found.Theirs}}
	for _, side := range sides {
		if side.rec == nil {
			continue
		}
		data, _, err := runGit(ctx, r.path, "show", ":"+strconv.Itoa(side.stage)+":"+clean)
		if err != nil {
			return nil, err
		}
		cc.Stages[side.stage] = data
	}
	working, readErr := os.ReadFile(filepath.Join(r.path, filepath.FromSlash(clean)))
	if readErr != nil {
		if os.IsNotExist(readErr) {
			return cc, nil
		}
		return nil, readErr
	}
	cc.Working, cc.WorkingExists = working, true
	return cc, nil
}
