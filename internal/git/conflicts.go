package git

import (
	"context"
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
