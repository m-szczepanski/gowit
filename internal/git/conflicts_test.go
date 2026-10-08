package git

import (
	"context"
	"strconv"
	"strings"
	"testing"
)

// conflictFixture merges two diverged branches so three conflict shapes
// coexist: a.txt content/content (UU), n.txt add/add (AA, no base),
// f.txt rename/delete (DU: side mv e.txt->f.txt, main rm e.txt; no ours
// stage, no working file).
func conflictFixture(t *testing.T) string {
	t.Helper()
	dir := initRepo(t)
	writeFile(t, dir, "a.txt", "base\n")
	writeFile(t, dir, "e.txt", "e\n")
	commitAll(t, dir, "base")

	gitOut(t, dir, "checkout", "-qb", "side")
	writeFile(t, dir, "a.txt", "side\n")
	writeFile(t, dir, "n.txt", "side\n")
	gitOut(t, dir, "add", "n.txt")
	gitOut(t, dir, "mv", "e.txt", "f.txt")
	commitAll(t, dir, "side work")

	gitOut(t, dir, "checkout", "-q", "main")
	writeFile(t, dir, "a.txt", "main\n")
	writeFile(t, dir, "n.txt", "main\n")
	gitOut(t, dir, "add", "n.txt")
	gitOut(t, dir, "rm", "-q", "e.txt")
	commitAll(t, dir, "main work")

	// git merge prints its conflict report on stdout; stderr stays empty
	stdout, _, err := runGit(context.Background(), dir, "merge", "--no-ff", "-m", "m", "side")
	if err == nil || !strings.Contains(string(stdout), "CONFLICT") {
		t.Fatalf("fixture merge must conflict, got err %v stdout %q", err, stdout)
	}
	return dir
}

func stageOIDs(t *testing.T, dir string) map[string]string {
	t.Helper()
	out := gitOut(t, dir, "ls-files", "-u")
	byPathStage := map[string]string{}
	for _, line := range strings.Split(out, "\n") {
		if line == "" {
			continue
		}
		meta, path, ok := strings.Cut(line, "\t")
		if !ok {
			t.Fatalf("unparsable ls-files line %q", line)
		}
		parts := strings.Fields(meta)
		if len(parts) != 3 {
			t.Fatalf("unparsable ls-files meta %q", meta)
		}
		byPathStage[path+"#"+parts[2]] = parts[1]
	}
	return byPathStage
}

func TestConflictsAllShapes(t *testing.T) {
	ctx := context.Background()
	dir := conflictFixture(t)
	r := openRepo(t, dir)

	list, err := r.Conflicts(ctx)
	if err != nil {
		t.Fatalf("Conflicts: %v", err)
	}
	if len(list) != 3 {
		t.Fatalf("len = %d, want 3: %+v", len(list), list)
	}
	if list[0].Path != "a.txt" || list[1].Path != "f.txt" || list[2].Path != "n.txt" {
		t.Fatalf("paths not sorted: %+v", list)
	}

	oids := stageOIDs(t, dir)
	byPath := map[string]Conflict{}
	for _, c := range list {
		byPath[c.Path] = c
	}

	a := byPath["a.txt"]
	if a.XY != "UU" {
		t.Fatalf("a.txt XY = %q, want UU", a.XY)
	}
	for stage, got := range map[int]*MergeStage{1: a.Base, 2: a.Ours, 3: a.Theirs} {
		want := oids["a.txt#"+strconv.Itoa(stage)]
		if got == nil || got.Oid != want {
			t.Fatalf("a.txt stage %d = %+v, want oid %s", stage, got, want)
		}
		if got.Stage != stage {
			t.Fatalf("stage number = %d, want %d", got.Stage, stage)
		}
	}

	f := byPath["f.txt"]
	if f.XY != "DU" {
		t.Fatalf("f.txt XY = %q, want DU", f.XY)
	}
	if f.Ours != nil {
		t.Fatalf("rename/delete must have no ours stage: %+v", f.Ours)
	}
	if f.Base == nil || f.Base.Oid != oids["f.txt#1"] {
		t.Fatalf("f.txt base = %+v, want %s", f.Base, oids["f.txt#1"])
	}
	if f.Theirs == nil || f.Theirs.Oid != oids["f.txt#3"] {
		t.Fatalf("f.txt theirs = %+v, want %s", f.Theirs, oids["f.txt#3"])
	}

	n := byPath["n.txt"]
	if n.XY != "AA" {
		t.Fatalf("n.txt XY = %q, want AA", n.XY)
	}
	if n.Base != nil {
		t.Fatalf("add/add must have no base stage: %+v", n.Base)
	}
	if n.Ours == nil || n.Ours.Oid != oids["n.txt#2"] || n.Theirs == nil || n.Theirs.Oid != oids["n.txt#3"] {
		t.Fatalf("n.txt stages = %+v/%+v, want %s/%s", n.Ours, n.Theirs, oids["n.txt#2"], oids["n.txt#3"])
	}
}

func TestConflictsMatchStatusView(t *testing.T) {
	ctx := context.Background()
	dir := conflictFixture(t)
	r := openRepo(t, dir)

	res, err := r.Status(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var fromStatus []string
	for _, f := range res.Files {
		if f.Conflict {
			fromStatus = append(fromStatus, f.Path)
		}
	}
	list, err := r.Conflicts(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var fromConflicts []string
	for _, c := range list {
		fromConflicts = append(fromConflicts, c.Path)
	}
	if strings.Join(fromStatus, ",") != strings.Join(fromConflicts, ",") {
		t.Fatalf("staging view %v differs from conflicts view %v", fromStatus, fromConflicts)
	}
}

func TestConflictsCleanRepo(t *testing.T) {
	dir := initRepo(t)
	writeFile(t, dir, "a.txt", "x\n")
	commitAll(t, dir, "c")
	list, err := openRepo(t, dir).Conflicts(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 0 {
		t.Fatalf("got %+v, want none", list)
	}
}
