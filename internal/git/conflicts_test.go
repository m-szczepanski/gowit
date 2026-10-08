package git

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// conflictFixture merges two diverged branches so three conflict shapes
// coexist: a.txt content/content (UU), n.txt add/add (AA, no base),
// f.txt rename/delete (DU: side mv e.txt->f.txt, main rm e.txt; stages
// 1 and 3 only, the surviving copy stays under the new name).
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

func conflictContentMap(t *testing.T, dir, path string) map[string]string {
	t.Helper()
	cc, err := openRepo(t, dir).ConflictStages(context.Background(), path)
	if err != nil {
		t.Fatalf("ConflictStages(%s): %v", path, err)
	}
	got := map[string]string{}
	for stage, data := range cc.Stages {
		got[strconv.Itoa(stage)] = string(data)
	}
	if cc.WorkingExists {
		got["working"] = string(cc.Working)
	}
	return got
}

func TestConflictStagesContentConflict(t *testing.T) {
	got := conflictContentMap(t, conflictFixture(t), "a.txt")
	want := map[string]string{"1": "base\n", "2": "main\n", "3": "side\n"}
	for k, v := range want {
		if got[k] != v {
			t.Fatalf("a.txt[%s] = %q, want %q", k, got[k], v)
		}
	}
	working := got["working"]
	if !strings.Contains(working, "<<<<<<<") || !strings.Contains(working, "main\n") || !strings.Contains(working, "side\n") {
		t.Fatalf("merged working file lacks markers or sides: %q", working)
	}
}

func TestConflictStagesAddAdd(t *testing.T) {
	got := conflictContentMap(t, conflictFixture(t), "n.txt")
	if _, ok := got["1"]; ok {
		t.Fatalf("add/add must have no base stage, got %q", got["1"])
	}
	if got["2"] != "main\n" || got["3"] != "side\n" {
		t.Fatalf("n.txt ours/theirs = %q/%q", got["2"], got["3"])
	}
}

func TestConflictStagesRenameDelete(t *testing.T) {
	dir := conflictFixture(t)
	cc, err := openRepo(t, dir).ConflictStages(context.Background(), "f.txt")
	if err != nil {
		t.Fatalf("ConflictStages(f.txt): %v", err)
	}
	// git leaves the surviving side's file in the work tree
	if !cc.WorkingExists || string(cc.Working) != "e\n" {
		t.Fatalf("f.txt working = %q exists=%v, want e present", cc.Working, cc.WorkingExists)
	}
	if string(cc.Stages[1]) != "e\n" || string(cc.Stages[3]) != "e\n" {
		t.Fatalf("f.txt stages = %v, want base and theirs = e", cc.Stages)
	}
	if _, ok := cc.Stages[2]; ok {
		t.Fatalf("f.txt must have no ours stage")
	}
}

func TestConflictStagesMissingWorkingFile(t *testing.T) {
	dir := conflictFixture(t)
	if err := os.Remove(filepath.Join(dir, "a.txt")); err != nil {
		t.Fatal(err)
	}
	cc, err := openRepo(t, dir).ConflictStages(context.Background(), "a.txt")
	if err != nil {
		t.Fatalf("ConflictStages after user deletes: %v", err)
	}
	if cc.WorkingExists || cc.Working != nil {
		t.Fatalf("absent file must read as missing, got exists=%v %q", cc.WorkingExists, cc.Working)
	}
	if len(cc.Stages) != 3 {
		t.Fatalf("stages survive the work-tree deletion, got %v", cc.Stages)
	}
}

func TestConflictStagesGuards(t *testing.T) {
	ctx := context.Background()
	dir := conflictFixture(t)
	r := openRepo(t, dir)

	notConflict := &GitError{Code: CodeValidationFailed}
	_, err := r.ConflictStages(ctx, "e.txt")
	ge, ok := err.(*GitError)
	if !ok || ge.Code != notConflict.Code || !strings.Contains(ge.Message, "not conflicted") {
		t.Fatalf("err = %v, want validation_failed not conflicted", err)
	}
	for _, bad := range []string{"", "../escape", "/abs"} {
		if _, err := r.ConflictStages(ctx, bad); !errors.Is(err, ErrValidationFailed) {
			t.Fatalf("ConflictStages(%q) = %v, want validation_failed", bad, err)
		}
	}
}

func TestConflictsCtxKill(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := openRepo(t, conflictFixture(t)).Conflicts(ctx); !errors.Is(err, ErrTimeout) {
		t.Fatalf("err = %v, want timeout", err)
	}
}

func TestConflictStagesCtxKill(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := openRepo(t, conflictFixture(t)).ConflictStages(ctx, "a.txt")
	if !errors.Is(err, ErrTimeout) {
		t.Fatalf("err = %v, want timeout", err)
	}
}

func TestConflictsSkipsNonConflictFiles(t *testing.T) {
	dir := conflictFixture(t)
	writeFile(t, dir, "plain.txt", "dirt\n")
	list, err := openRepo(t, dir).Conflicts(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 3 {
		t.Fatalf("dirty non-conflict file leaked into conflicts: %+v", list)
	}
}

func TestConflictStagesUnreadableWorkingPath(t *testing.T) {
	skipWithoutUnixPerms(t)
	dir := conflictFixture(t)
	victim := filepath.Join(dir, "a.txt")
	if err := os.Remove(victim); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(victim, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Remove(victim) })
	_, err := openRepo(t, dir).ConflictStages(context.Background(), "a.txt")
	if err == nil || os.IsNotExist(err) {
		t.Fatalf("err = %v, want the directory read error propagated", err)
	}
}

func TestConflictStagesUnreadableObjectPropagates(t *testing.T) {
	skipWithoutUnixPerms(t)
	dir := conflictFixture(t)
	oid := gitOut(t, dir, "rev-parse", ":2:a.txt")
	obj := filepath.Join(dir, ".git", "objects", oid[:2], oid[2:])
	if err := os.Chmod(obj, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(obj, 0o444) })
	_, err := openRepo(t, dir).ConflictStages(context.Background(), "a.txt")
	if !errors.Is(err, ErrCommandFailed) {
		t.Fatalf("err = %v, want git show failure propagated", err)
	}
}

func TestConflictOperationCleanRepo(t *testing.T) {
	op, err := openRepo(t, initRepo(t)).ConflictOperation(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if op != OperationNone {
		t.Fatalf("op = %q, want none", op)
	}
}

func TestConflictOperationMerge(t *testing.T) {
	op, err := openRepo(t, conflictFixture(t)).ConflictOperation(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if op != OperationMerge {
		t.Fatalf("op = %q, want merge", op)
	}
}

func TestConflictOperationMergePersistsAfterResolvingIndex(t *testing.T) {
	ctx := context.Background()
	dir := conflictFixture(t)
	writeFile(t, dir, "a.txt", "fixed\n")
	gitOut(t, dir, "add", "a.txt")
	op, err := openRepo(t, dir).ConflictOperation(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if op != OperationMerge {
		t.Fatalf("op = %q after staging a resolution, want merge until the operation ends", op)
	}
}

func TestConflictOperationCherryPick(t *testing.T) {
	ctx := context.Background()
	dir, oids := conflictPickRepo(t)
	r := openRepo(t, dir)
	if err := r.CherryPick(ctx, oids[:2], CherryPickOptions{}); !errors.Is(err, ErrCherryPickConflict) {
		t.Fatal(err)
	}
	op, err := r.ConflictOperation(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if op != OperationCherryPick {
		t.Fatalf("op = %q, want cherry-pick", op)
	}
}

// rebaseConflictRepo stops a main-onto-topic rebase on a content conflict.
func rebaseConflictRepo(t *testing.T, extra ...string) string {
	t.Helper()
	dir := initRepo(t)
	writeFile(t, dir, "a.txt", "base\n")
	commitAll(t, dir, "base")
	gitOut(t, dir, "checkout", "-qb", "topic")
	writeFile(t, dir, "a.txt", "topic\n")
	commitAll(t, dir, "topic change")
	gitOut(t, dir, "checkout", "-q", "main")
	writeFile(t, dir, "a.txt", "main\n")
	commitAll(t, dir, "main change")
	args := append([]string{"rebase"}, extra...)
	_, _, err := runGit(context.Background(), dir, append(args, "topic")...)
	if err == nil {
		t.Fatal("fixture rebase must stop conflicted")
	}
	return dir
}

func TestConflictOperationRebase(t *testing.T) {
	for _, backend := range [][]string{nil, {"--apply"}} {
		dir := rebaseConflictRepo(t, backend...)
		op, err := openRepo(t, dir).ConflictOperation(context.Background())
		if err != nil {
			t.Fatalf("backend %v: %v", backend, err)
		}
		if op != OperationRebase {
			t.Fatalf("backend %v op = %q, want rebase", backend, op)
		}
	}
}

func TestConflictOperationStatFailurePropagates(t *testing.T) {
	skipWithoutUnixPerms(t)
	r := openRepo(t, initRepo(t))
	gitdir := filepath.Join(r.path, ".git")
	if err := os.Chmod(gitdir, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(gitdir, 0o755) })
	if _, err := r.ConflictOperation(context.Background()); err == nil {
		t.Fatal("want stat failure propagated")
	}
}

func TestConflictOperationCtxKill(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := openRepo(t, conflictFixture(t)).ConflictOperation(ctx); !errors.Is(err, ErrTimeout) {
		t.Fatalf("err = %v, want timeout", err)
	}
}
