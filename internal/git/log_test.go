package git

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLogLinearHistory(t *testing.T) {
	ctx := context.Background()
	dir := initRepo(t)
	writeFile(t, dir, "f.txt", "x\n")
	commitAll(t, dir, "first\n\nbody A\nbody B")
	writeFile(t, dir, "f.txt", "y\n")
	commitAll(t, dir, "ünïcode ✨ subject")
	writeFile(t, dir, "f.txt", "z\n")
	commitAll(t, dir, "last")
	runGit(ctx, dir, "tag", "v0.1")

	commits, err := openRepo(t, dir).Log(ctx, LogOptions{})
	if err != nil {
		t.Fatalf("Log: %v", err)
	}
	if len(commits) != 3 {
		t.Fatalf("got %d commits, want 3", len(commits))
	}
	first, middle, latest := commits[2], commits[1], commits[0]

	if first.Hash != hashOf(t, dir, "HEAD~2") || first.Subject != "first" || first.Body != "body A\nbody B" {
		t.Fatalf("first = %+v", first)
	}
	if len(first.ParentHashes) != 0 {
		t.Fatalf("root parents = %v, want none", first.ParentHashes)
	}
	if !strings.HasPrefix(first.Hash, first.ShortHash) || len(first.ShortHash) < 7 || first.ShortHash != first.Hash[:len(first.ShortHash)] {
		t.Fatalf("short hash %q inconsistent with %q", first.ShortHash, first.Hash)
	}
	if middle.Subject != "ünïcode ✨ subject" || middle.Body != "" {
		t.Fatalf("middle = %+v, want unicode subject, empty body", middle)
	}
	if latest.Hash != hashOf(t, dir, "HEAD") || len(latest.ParentHashes) != 1 || latest.ParentHashes[0] != middle.Hash {
		t.Fatalf("latest = %+v, want one parent = middle", latest)
	}
	if got := latest.Refs; len(got) != 2 || got[0] != (Ref{Kind: RefHead, Name: "main"}) || got[1] != (Ref{Kind: RefTag, Name: "v0.1"}) {
		t.Fatalf("latest refs = %+v, want HEAD->main and tag v0.1", got)
	}
	if latest.AuthorName != "test" || latest.AuthorEmail != "t@t" {
		t.Fatalf("identity = %q %q", latest.AuthorName, latest.AuthorEmail)
	}
	if latest.AuthorDate.IsZero() || !latest.AuthorDate.Equal(latest.CommitterDate) {
		t.Fatalf("dates = %v %v, want equal non-zero", latest.AuthorDate, latest.CommitterDate)
	}
	if d := time.Since(latest.AuthorDate); d < -time.Minute || d > time.Minute {
		t.Fatalf("author date %v not near now", latest.AuthorDate)
	}
}

func TestLogUnbornRepoIsEmpty(t *testing.T) {
	dir := initRepo(t)
	commits, err := openRepo(t, dir).Log(context.Background(), LogOptions{})
	if err != nil {
		t.Fatalf("Log: %v", err)
	}
	if len(commits) != 0 {
		t.Fatalf("got %+v, want empty", commits)
	}
}

func TestLogCtxKill(t *testing.T) {
	dir := initRepo(t)
	writeFile(t, dir, "f.txt", "x\n")
	commitAll(t, dir, "one")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := openRepo(t, dir).Log(ctx, LogOptions{}); !errors.Is(err, ErrTimeout) {
		t.Fatalf("err = %v, want timeout", err)
	}
}

func TestLogPropagatesGitFailure(t *testing.T) {
	ctx := context.Background()
	dir := initRepo(t)
	writeFile(t, dir, "f.txt", "x\n")
	commitAll(t, dir, "root")
	writeFile(t, dir, "f.txt", "y\n")
	commitAll(t, dir, "tip")

	// HEAD itself resolves, so the unborn guard passes, but walking parents
	// reads the corrupted root commit object and git dies mid-log
	hash := hashOf(t, dir, "HEAD~1")
	object := filepath.Join(dir, ".git", "objects", hash[:2], hash[2:])
	// loose objects are committed read-only
	if err := os.Chmod(object, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(object, []byte("garbage-not-a-commit"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := openRepo(t, dir).Log(ctx, LogOptions{}); !errors.Is(err, ErrCommandFailed) {
		t.Fatalf("err = %v, want command_failed", err)
	}
}

func TestLogPagination(t *testing.T) {
	ctx := context.Background()
	dir := initRepo(t)
	writeFile(t, dir, "f.txt", "0\n")
	commitAll(t, dir, "c0")
	for i := 1; i < 5; i++ {
		writeFile(t, dir, "f.txt", fmt.Sprint(i)+"\n")
		commitAll(t, dir, fmt.Sprint("c", i))
	}
	repo := openRepo(t, dir)
	all, err := repo.Log(ctx, LogOptions{})
	if err != nil || len(all) != 5 {
		t.Fatalf("all = %d commits err %v, want 5", len(all), err)
	}

	page, err := repo.Log(ctx, LogOptions{MaxCount: 2})
	if err != nil {
		t.Fatal(err)
	}
	if len(page) != 2 || page[0].Subject != "c4" || page[1].Subject != "c3" {
		t.Fatalf("first page = %+v, want c4, c3", subjects(page))
	}

	page, err = repo.Log(ctx, LogOptions{MaxCount: 2, Skip: 2})
	if err != nil {
		t.Fatal(err)
	}
	if len(page) != 2 || page[0].Subject != "c2" || page[1].Subject != "c1" {
		t.Fatalf("second page = %+v, want c2, c1", subjects(page))
	}

	// window reaching past the oldest commit clamps instead of failing
	page, err = repo.Log(ctx, LogOptions{MaxCount: 2, Skip: 4})
	if err != nil {
		t.Fatal(err)
	}
	if len(page) != 1 || page[0].Subject != "c0" {
		t.Fatalf("tail page = %+v, want just c0", subjects(page))
	}

	page, err = repo.Log(ctx, LogOptions{Skip: 50})
	if err != nil {
		t.Fatal(err)
	}
	if len(page) != 0 {
		t.Fatalf("past-end page = %+v, want empty", subjects(page))
	}

	if _, err := repo.Log(ctx, LogOptions{MaxCount: -1}); !errors.Is(err, ErrValidationFailed) {
		t.Fatalf("err = %v, want validation_failed", err)
	}
	if _, err := repo.Log(ctx, LogOptions{Skip: -2}); !errors.Is(err, ErrValidationFailed) {
		t.Fatalf("err = %v, want validation_failed", err)
	}
}

func subjects(cs []Commit) []string {
	out := []string{}
	for _, c := range cs {
		out = append(out, c.Subject)
	}
	return out
}

func TestLogFilters(t *testing.T) {
	ctx := context.Background()
	dir, mergeHash, mainHash, _ := mergeRepo(t)
	// history: root(base.txt) -> side-add(s.txt) on branch side ->
	// main-edit(base.txt) on main -> merged (first parent main-edit)
	repo := openRepo(t, dir)

	ref, err := repo.Log(ctx, LogOptions{Ref: "side"})
	if err != nil {
		t.Fatalf("Ref=side: %v", err)
	}
	if len(ref) != 2 || ref[0].Subject != "side add" || ref[1].Subject != "root" {
		t.Fatalf("Ref=side = %+v, want side add, root", subjects(ref))
	}
	byHash, err := repo.Log(ctx, LogOptions{Ref: mainHash})
	if err != nil {
		t.Fatalf("Ref=hash: %v", err)
	}
	if len(byHash) != 2 || byHash[0].Subject != "main edit" {
		t.Fatalf("Ref=hash = %+v, want main edit, root", subjects(byHash))
	}

	fp, err := repo.Log(ctx, LogOptions{FirstParent: true})
	if err != nil {
		t.Fatalf("FirstParent: %v", err)
	}
	if len(fp) != 3 || fp[0].Subject != "merged" || fp[1].Subject != "main edit" || fp[2].Subject != "root" {
		t.Fatalf("FirstParent = %+v, want merged, main edit, root (side add hidden)", subjects(fp))
	}

	all, err := repo.Log(ctx, LogOptions{All: true})
	if err != nil {
		t.Fatalf("All: %v", err)
	}
	if len(all) != 4 {
		t.Fatalf("All = %+v, want all four commits", subjects(all))
	}

	path, err := repo.Log(ctx, LogOptions{Path: "s.txt"})
	if err != nil {
		t.Fatalf("Path=s.txt: %v", err)
	}
	if len(path) != 1 || path[0].Subject != "side add" {
		t.Fatalf("Path=s.txt = %+v, want side add only", subjects(path))
	}

	comb, err := repo.Log(ctx, LogOptions{Ref: mergeHash, Path: "base.txt", MaxCount: 1})
	if err != nil {
		t.Fatalf("combined: %v", err)
	}
	if len(comb) != 1 || comb[0].Subject != "main edit" {
		t.Fatalf("combined = %+v, want main edit only", subjects(comb))
	}
}

func TestLogFilterValidation(t *testing.T) {
	ctx := context.Background()
	dir, _, _, _ := mergeRepo(t)
	repo := openRepo(t, dir)

	if _, err := repo.Log(ctx, LogOptions{Ref: "-x"}); !errors.Is(err, ErrValidationFailed) {
		t.Fatalf("err = %v, want validation_failed", err)
	}
	if _, err := repo.Log(ctx, LogOptions{Path: "../escape"}); !errors.Is(err, ErrValidationFailed) {
		t.Fatalf("err = %v, want validation_failed", err)
	}
	if _, err := repo.Log(ctx, LogOptions{Path: "/abs"}); !errors.Is(err, ErrValidationFailed) {
		t.Fatalf("err = %v, want validation_failed", err)
	}
}

func TestLogUnbornWithAllIsEmpty(t *testing.T) {
	dir := initRepo(t)
	commits, err := openRepo(t, dir).Log(context.Background(), LogOptions{All: true})
	if err != nil || len(commits) != 0 {
		t.Fatalf("got %+v err %v, want empty", commits, err)
	}
}

func TestCommitChangedFiles(t *testing.T) {
	ctx := context.Background()
	dir := initRepo(t)
	writeFile(t, dir, "a.txt", "one\n")
	writeFile(t, dir, "g.txt", "rename source content\nkept\nacross\nthe boundary\n")
	writeFile(t, dir, "del.txt", "bye\n")
	commitAll(t, dir, "root")
	writeFile(t, dir, "a.txt", "two\n")
	runGit(ctx, dir, "mv", "g.txt", "h.txt")
	runGit(ctx, dir, "rm", "del.txt")
	commitAll(t, dir, "changes")

	repo := openRepo(t, dir)
	files, err := repo.CommitChangedFiles(ctx, hashOf(t, dir, "HEAD"))
	if err != nil {
		t.Fatalf("CommitChangedFiles: %v", err)
	}
	m := map[string]CommitFileStat{}
	for _, f := range files {
		m[f.Path] = f
	}
	if len(files) != 3 || len(m) != 3 {
		t.Fatalf("got %+v, want three entries", files)
	}
	if s := m["a.txt"]; s.Change != ChangeModified {
		t.Fatalf("a.txt = %+v, want modified", s)
	}
	if s := m["del.txt"]; s.Change != ChangeDeleted {
		t.Fatalf("del.txt = %+v, want deleted", s)
	}
	if s := m["h.txt"]; s.Change != ChangeRenamed || s.OldPath != "g.txt" || s.Similarity != 100 {
		t.Fatalf("h.txt = %+v, want R100 of g.txt", s)
	}

	// same first-parent rule as DiffCommitFile/DiffCommitFiles
	mdir, mergeHash, _, _ := mergeRepo(t)
	files, err = openRepo(t, mdir).CommitChangedFiles(ctx, mergeHash)
	if err != nil || len(files) != 1 || files[0].Change != ChangeAdded || files[0].Path != "s.txt" {
		t.Fatalf("merge = %+v err %v, want only s.txt added", files, err)
	}

	if _, err := repo.CommitChangedFiles(ctx, ""); !errors.Is(err, ErrValidationFailed) {
		t.Fatalf("err = %v, want validation_failed", err)
	}
	if _, err := repo.CommitChangedFiles(ctx, "nope-not-a-ref"); !errors.Is(err, ErrCommandFailed) {
		t.Fatalf("err = %v, want command_failed", err)
	}
	kctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := repo.CommitChangedFiles(kctx, "HEAD"); !errors.Is(err, ErrTimeout) {
		t.Fatalf("err = %v, want timeout", err)
	}
}

func TestLogMergeOctopusAndEmptyCommits(t *testing.T) {
	ctx := context.Background()
	dir := initRepo(t)
	writeFile(t, dir, "a.txt", "base\n")
	commitAll(t, dir, "base")
	baseHash := hashOf(t, dir, "HEAD")

	for i := 1; i <= 3; i++ {
		runGit(ctx, dir, "checkout", "-qb", fmt.Sprintf("s%d", i), baseHash)
		writeFile(t, dir, fmt.Sprintf("f%d.txt", i), fmt.Sprint(i)+"\n")
		commitAll(t, dir, fmt.Sprintf("side %d", i))
		runGit(ctx, dir, "checkout", "-q", "main")
	}

	runGit(ctx, dir, "commit", "-q", "--allow-empty", "-m", "empty commit")
	if _, _, err := runGit(ctx, dir, "merge", "-q", "-m", "two-parent", "s1"); err != nil {
		t.Fatal(err)
	}
	two := hashOf(t, dir, "HEAD")
	if _, _, err := runGit(ctx, dir, "merge", "-q", "-m", "octopus", "s2", "s3"); err != nil {
		t.Fatal(err)
	}
	octopus := hashOf(t, dir, "HEAD")

	commits, err := openRepo(t, dir).Log(ctx, LogOptions{})
	if err != nil {
		t.Fatalf("Log: %v", err)
	}
	byHash := map[string]Commit{}
	for _, c := range commits {
		byHash[c.Hash] = c
	}

	var empty *Commit
	for i := range commits {
		if commits[i].Subject == "empty commit" {
			empty = &commits[i]
		}
	}
	if empty == nil {
		t.Fatal("empty commit missing from log")
	}
	if len(empty.ParentHashes) != 1 || empty.ParentHashes[0] != baseHash {
		t.Fatalf("empty commit parents = %v, want [base]", empty.ParentHashes)
	}

	twoC := byHash[two]
	if len(twoC.ParentHashes) != 2 {
		t.Fatalf("two-parent merge parents = %v", twoC.ParentHashes)
	}
	if twoC.ParentHashes[0] == baseHash {
		t.Fatalf("two-parent first parent = %v, want the pre-merge main tip, not base", twoC.ParentHashes[0])
	}
	if twoC.ParentHashes[1] != hashOf(t, dir, "s1") {
		t.Fatalf("two-parent second parent = %v, want s1", twoC.ParentHashes[1])
	}

	oc := byHash[octopus]
	if len(oc.ParentHashes) != 3 || oc.ParentHashes[0] != two ||
		oc.ParentHashes[1] != hashOf(t, dir, "s2") || oc.ParentHashes[2] != hashOf(t, dir, "s3") {
		t.Fatalf("octopus parents = %v, want [two, s2, s3]", oc.ParentHashes)
	}

	fp, err := openRepo(t, dir).Log(ctx, LogOptions{FirstParent: true})
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range fp {
		if strings.HasPrefix(c.Subject, "side ") {
			t.Fatalf("first-parent log leaked side branch commit %q", c.Subject)
		}
	}
	if len(fp) != 4 {
		t.Fatalf("first-parent log = %+v, want octopus, two-parent, empty, base", subjects(fp))
	}
}
