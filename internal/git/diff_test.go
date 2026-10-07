package git

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func openRepo(t *testing.T, dir string) *Repo {
	t.Helper()
	repo, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	return repo
}

func diffWorking(t *testing.T, dir, path string, staged bool) (*FileDiff, error) {
	t.Helper()
	return openRepo(t, dir).DiffWorkingFile(context.Background(), path, staged)
}

func gitConfig(t *testing.T, dir, key, value string) {
	t.Helper()
	if _, _, err := runGit(context.Background(), dir, "config", key, value); err != nil {
		t.Fatal(err)
	}
}

func TestDiffWorkingFileUnstagedModify(t *testing.T) {
	dir := initRepo(t)
	writeFile(t, dir, "a.txt", "one\ntwo\n")
	commitAll(t, dir, "base")
	writeFile(t, dir, "a.txt", "one\nTWO\n")

	fd, err := diffWorking(t, dir, "a.txt", false)
	if err != nil {
		t.Fatalf("DiffWorkingFile: %v", err)
	}
	if fd.Change != ChangeModified || fd.OldPath != "a.txt" || fd.NewPath != "a.txt" {
		t.Fatalf("got %+v, want modified a.txt", fd)
	}
	if len(fd.Hunks) != 1 || fd.Hunks[0].Header != "@@ -1,2 +1,2 @@" {
		t.Fatalf("hunks = %+v, want one hunk with git default header", fd.Hunks)
	}
	lines := fd.Hunks[0].Lines
	if len(lines) != 3 || lines[1] != (DiffLine{Type: DiffLineDel, OldNum: 2, Text: "two"}) ||
		lines[2] != (DiffLine{Type: DiffLineAdd, NewNum: 2, Text: "TWO"}) {
		t.Fatalf("lines = %+v, want ctx/del/add with worked numbers", lines)
	}
}

func TestDiffWorkingFileStaged(t *testing.T) {
	dir := initRepo(t)
	writeFile(t, dir, "a.txt", "one\ntwo\n")
	commitAll(t, dir, "base")
	writeFile(t, dir, "a.txt", "one\nTWO\n")
	if _, _, err := runGit(context.Background(), dir, "add", "a.txt"); err != nil {
		t.Fatal(err)
	}

	staged, err := diffWorking(t, dir, "a.txt", true)
	if err != nil {
		t.Fatalf("staged diff: %v", err)
	}
	if len(staged.Hunks) != 1 || staged.Change != ChangeModified {
		t.Fatalf("staged = %+v, want modified with a hunk", staged)
	}
	// after staging, the work tree matches the index: unstaged side is empty
	unstaged, err := diffWorking(t, dir, "a.txt", false)
	if err != nil {
		t.Fatalf("unstaged diff: %v", err)
	}
	if len(unstaged.Hunks) != 0 || unstaged.Change != ChangeModified || unstaged.NewPath != "a.txt" {
		t.Fatalf("unstaged = %+v, want unchanged a.txt", unstaged)
	}
}

func TestDiffWorkingFileStagedAdd(t *testing.T) {
	dir := initRepo(t)
	writeFile(t, dir, "seed.txt", "s\n")
	commitAll(t, dir, "base")
	writeFile(t, dir, "new.txt", "hello\n")
	if _, _, err := runGit(context.Background(), dir, "add", "new.txt"); err != nil {
		t.Fatal(err)
	}

	fd, err := diffWorking(t, dir, "new.txt", true)
	if err != nil {
		t.Fatalf("DiffWorkingFile: %v", err)
	}
	if fd.Change != ChangeAdded || fd.OldPath != "" || fd.NewPath != "new.txt" {
		t.Fatalf("got %+v, want added new.txt", fd)
	}
	if len(fd.Hunks) != 1 || fd.Hunks[0].Header != "@@ -0,0 +1 @@" {
		t.Fatalf("hunks = %+v, want one added hunk", fd.Hunks)
	}
}

func TestDiffWorkingFileWorktreeDelete(t *testing.T) {
	dir := initRepo(t)
	writeFile(t, dir, "a.txt", "one\ntwo\n")
	commitAll(t, dir, "base")
	if err := os.Remove(filepath.Join(dir, "a.txt")); err != nil {
		t.Fatal(err)
	}

	fd, err := diffWorking(t, dir, "a.txt", false)
	if err != nil {
		t.Fatalf("DiffWorkingFile: %v", err)
	}
	if fd.Change != ChangeDeleted || fd.OldPath != "a.txt" || fd.NewPath != "" {
		t.Fatalf("got %+v, want deleted a.txt", fd)
	}
	if len(fd.Hunks) != 1 || fd.Hunks[0].Header != "@@ -1,2 +0,0 @@" {
		t.Fatalf("hunks = %+v, want one deletion hunk", fd.Hunks)
	}
}

func TestDiffWorkingFileUntrackedSynthesized(t *testing.T) {
	dir := initRepo(t)
	writeFile(t, dir, "seed.txt", "s\n")
	commitAll(t, dir, "base")
	writeFile(t, dir, "u.txt", "x\ny\nz")

	fd, err := diffWorking(t, dir, "u.txt", false)
	if err != nil {
		t.Fatalf("DiffWorkingFile: %v", err)
	}
	if fd.Change != ChangeAdded || fd.OldPath != "" || fd.NewPath != "u.txt" {
		t.Fatalf("got %+v, want added u.txt", fd)
	}
	if len(fd.Hunks) != 1 {
		t.Fatalf("hunks = %+v, want exactly one", fd.Hunks)
	}
	h := fd.Hunks[0]
	if h.Header != "@@ -0,0 +1,3 @@" || h.OldStart != 0 || h.OldCount != 0 || h.NewStart != 1 || h.NewCount != 3 {
		t.Fatalf("hunk = %+v, want -0,0 +1,3", h)
	}
	wantLines := []DiffLine{
		{Type: DiffLineAdd, NewNum: 1, Text: "x"},
		{Type: DiffLineAdd, NewNum: 2, Text: "y"},
		{Type: DiffLineAdd, NewNum: 3, Text: "z", NoNewline: true},
	}
	for i := range wantLines {
		if h.Lines[i] != wantLines[i] {
			t.Fatalf("line %d = %+v, want %+v", i, h.Lines[i], wantLines[i])
		}
	}
	if len(h.Lines) != 3 {
		t.Fatalf("got %d lines, want 3", len(h.Lines))
	}

	// the synthesized hunk header must be legal git syntax the parser accepts
	reparsed, err := parseUnifiedDiff(diffJoin(
		"diff --git a/u.txt b/u.txt",
		"new file mode 100644",
		"--- /dev/null",
		"+++ b/u.txt",
		h.Header,
		"+x", "+y", "+z",
		`\ No newline at end of file`,
	))
	if err != nil {
		t.Fatalf("round-trip parse: %v", err)
	}
	if len(reparsed) != 1 || !reflect.DeepEqual(reparsed[0].Hunks, []DiffHunk{h}) {
		t.Fatalf("round-trip mismatch: %+v vs %+v", reparsed[0].Hunks, h)
	}
}

func TestDiffWorkingFileUntrackedEmptyFile(t *testing.T) {
	dir := initRepo(t)
	writeFile(t, dir, "seed.txt", "s\n")
	commitAll(t, dir, "base")
	writeFile(t, dir, "e.txt", "")

	fd, err := diffWorking(t, dir, "e.txt", false)
	if err != nil {
		t.Fatalf("DiffWorkingFile: %v", err)
	}
	if fd.Change != ChangeAdded || fd.NewPath != "e.txt" || len(fd.Hunks) != 0 {
		t.Fatalf("got %+v, want added e.txt with no hunks (git shape)", fd)
	}
}

func TestDiffWorkingFileUntrackedBinary(t *testing.T) {
	dir := initRepo(t)
	writeFile(t, dir, "seed.txt", "s\n")
	commitAll(t, dir, "base")
	writeFileBytes(t, dir, "b.dat", []byte{0x00, 0xff, 'a'})

	fd, err := diffWorking(t, dir, "b.dat", false)
	if err != nil {
		t.Fatalf("DiffWorkingFile: %v", err)
	}
	if !fd.Binary || fd.Change != ChangeAdded || len(fd.Hunks) != 0 {
		t.Fatalf("got %+v, want binary added with no hunks", fd)
	}
}

func TestDiffWorkingFileUntrackedNulAfter8000BytesIsText(t *testing.T) {
	// git's rule: only the first 8000 bytes decide binary-ness
	dir := initRepo(t)
	writeFile(t, dir, "seed.txt", "s\n")
	commitAll(t, dir, "base")
	writeFileBytes(t, dir, "big.txt", []byte(strings.Repeat("a", 8001)+"\x00"))

	fd, err := diffWorking(t, dir, "big.txt", false)
	if err != nil {
		t.Fatalf("DiffWorkingFile: %v", err)
	}
	if fd.Binary || fd.Hunks[0].Lines[0].Text != strings.Repeat("a", 8001)+"\x00" {
		t.Fatalf("got binary=%v text=%q, want text with the NUL kept", fd.Binary, fd.Hunks[0].Lines[0].Text)
	}
}

func TestDiffWorkingFileUntrackedIgnoresStagedSide(t *testing.T) {
	dir := initRepo(t)
	writeFile(t, dir, "seed.txt", "s\n")
	commitAll(t, dir, "base")
	writeFile(t, dir, "u.txt", "x\n")

	// nothing is staged for an untracked file: the staged view is unchanged
	fd, err := diffWorking(t, dir, "u.txt", true)
	if err != nil {
		t.Fatalf("DiffWorkingFile: %v", err)
	}
	if len(fd.Hunks) != 0 || fd.Change != ChangeModified || fd.Binary {
		t.Fatalf("got %+v, want unchanged, never a synthesized add", fd)
	}
}

func TestDiffWorkingFileUntrackedDirectoryRejected(t *testing.T) {
	dir := initRepo(t)
	writeFile(t, dir, "seed.txt", "s\n")
	commitAll(t, dir, "base")
	mkdir(t, dir, "sub")
	writeFile(t, dir, "sub/nested.txt", "n\n")

	_, err := diffWorking(t, dir, "sub", false)
	if !errors.Is(err, ErrValidationFailed) {
		t.Fatalf("err = %v, want validation_failed", err)
	}
}

func TestDiffWorkingFileMultiFilePathspecRejected(t *testing.T) {
	dir := initRepo(t)
	mkdir(t, dir, "sub")
	writeFile(t, dir, "sub/a.txt", "a\n")
	writeFile(t, dir, "sub/b.txt", "b\n")
	commitAll(t, dir, "base")
	writeFile(t, dir, "sub/a.txt", "A\n")
	writeFile(t, dir, "sub/b.txt", "B\n")

	_, err := diffWorking(t, dir, "sub", false)
	if !errors.Is(err, ErrValidationFailed) {
		t.Fatalf("err = %v, want validation_failed", err)
	}
}

func TestDiffWorkingFileMissingPath(t *testing.T) {
	dir := initRepo(t)
	writeFile(t, dir, "seed.txt", "s\n")
	commitAll(t, dir, "base")

	_, err := diffWorking(t, dir, "ghost.txt", false)
	if !errors.Is(err, ErrPathMissing) {
		t.Fatalf("err = %v, want path_missing", err)
	}
}

func TestDiffWorkingFileRejectsBadPaths(t *testing.T) {
	dir := initRepo(t)
	writeFile(t, dir, "seed.txt", "s\n")
	commitAll(t, dir, "base")

	for _, path := range []string{"", ".", "/absolute/x", "../outside", "sub/../.."} {
		_, err := diffWorking(t, dir, path, false)
		if !errors.Is(err, ErrValidationFailed) {
			t.Fatalf("path %q: err = %v, want validation_failed", path, err)
		}
	}
}

func TestDiffWorkingFileLiteralGlobChars(t *testing.T) {
	dir := initRepo(t)
	writeFile(t, dir, "weird1.txt", "1\n")
	writeFile(t, dir, "weird*.txt", "2\n")
	commitAll(t, dir, "base")
	writeFile(t, dir, "weird1.txt", "one\n")
	writeFile(t, dir, "weird*.txt", "two\n")

	fd, err := diffWorking(t, dir, "weird*.txt", false)
	if err != nil {
		t.Fatalf("DiffWorkingFile: %v", err)
	}
	if fd.NewPath != "weird*.txt" || len(fd.Hunks) != 1 {
		t.Fatalf("got %+v, want only the literal weird*.txt", fd)
	}
}

func TestDiffWorkingFileIgnoresUserDiffConfig(t *testing.T) {
	dir := initRepo(t)
	mkdir(t, dir, "sub")
	writeFile(t, dir, "sub/x.txt", "old\n")
	commitAll(t, dir, "base")
	writeFile(t, dir, "sub/x.txt", "new\n")
	gitConfig(t, dir, "diff.noprefix", "true")
	gitConfig(t, dir, "diff.mnemonicPrefix", "true")

	fd, err := diffWorking(t, dir, "sub/x.txt", false)
	if err != nil {
		t.Fatalf("DiffWorkingFile: %v", err)
	}
	if fd.OldPath != "sub/x.txt" || fd.NewPath != "sub/x.txt" {
		t.Fatalf("paths = %q %q, want sub/x.txt despite user prefix config", fd.OldPath, fd.NewPath)
	}
}

func writeFileBytes(t *testing.T, dir, name string, data []byte) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func mkdir(t *testing.T, dir, name string) {
	t.Helper()
	if err := os.Mkdir(filepath.Join(dir, name), 0o755); err != nil {
		t.Fatal(err)
	}
}

func TestDiffWorkingFileModeOnlyStaged(t *testing.T) {
	dir := initRepo(t)
	writeFile(t, dir, "a.txt", "one\n")
	commitAll(t, dir, "base")
	if err := os.Chmod(filepath.Join(dir, "a.txt"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, _, err := runGit(context.Background(), dir, "add", "a.txt"); err != nil {
		t.Fatal(err)
	}

	fd, err := diffWorking(t, dir, "a.txt", true)
	if err != nil {
		t.Fatalf("DiffWorkingFile: %v", err)
	}
	// git's mode-only section carries no ---/+++ lines; paths must be
	// filled from the requested path
	if fd.Change != ChangeModified || fd.OldMode != "100644" || fd.NewMode != "100755" ||
		fd.OldPath != "a.txt" || fd.NewPath != "a.txt" || len(fd.Hunks) != 0 {
		t.Fatalf("got %+v, want mode-only a.txt 100644 -> 100755", fd)
	}
}

func TestDiffWorkingFileUntrackedTrailingNewline(t *testing.T) {
	dir := initRepo(t)
	writeFile(t, dir, "seed.txt", "s\n")
	commitAll(t, dir, "base")
	writeFile(t, dir, "u.txt", "a\nb\n")

	fd, err := diffWorking(t, dir, "u.txt", false)
	if err != nil {
		t.Fatalf("DiffWorkingFile: %v", err)
	}
	lines := fd.Hunks[0].Lines
	if len(lines) != 2 || lines[1].NoNewline || lines[1] != (DiffLine{Type: DiffLineAdd, NewNum: 2, Text: "b"}) {
		t.Fatalf("lines = %+v, want two adds, none flagged", lines)
	}
}

func TestDiffWorkingFileStagedZeroByteAdd(t *testing.T) {
	dir := initRepo(t)
	writeFile(t, dir, "seed.txt", "s\n")
	commitAll(t, dir, "base")
	writeFile(t, dir, "e.txt", "")
	if _, _, err := runGit(context.Background(), dir, "add", "e.txt"); err != nil {
		t.Fatal(err)
	}

	// git renders a zero-byte add as a header-only section: paths must be
	// filled from the requested path
	fd, err := diffWorking(t, dir, "e.txt", true)
	if err != nil {
		t.Fatalf("DiffWorkingFile: %v", err)
	}
	if fd.Change != ChangeAdded || fd.OldPath != "" || fd.NewPath != "e.txt" || len(fd.Hunks) != 0 {
		t.Fatalf("got %+v, want added e.txt with no hunks", fd)
	}
}

func TestDiffWorkingFileZeroByteDelete(t *testing.T) {
	dir := initRepo(t)
	writeFile(t, dir, "z.txt", "")
	commitAll(t, dir, "base")
	if err := os.Remove(filepath.Join(dir, "z.txt")); err != nil {
		t.Fatal(err)
	}

	fd, err := diffWorking(t, dir, "z.txt", false)
	if err != nil {
		t.Fatalf("DiffWorkingFile: %v", err)
	}
	if fd.Change != ChangeDeleted || fd.OldPath != "z.txt" || fd.NewPath != "" || len(fd.Hunks) != 0 {
		t.Fatalf("got %+v, want deleted z.txt with no hunks", fd)
	}
}

func TestDiffWorkingFileCtxKill(t *testing.T) {
	dir := initRepo(t)
	writeFile(t, dir, "a.txt", "x\n")
	commitAll(t, dir, "base")
	writeFile(t, dir, "a.txt", "y\n")

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := openRepo(t, dir).DiffWorkingFile(ctx, "a.txt", false)
	if !errors.Is(err, ErrTimeout) {
		t.Fatalf("err = %v, want timeout", err)
	}
}

func TestIsTrackedCtxKill(t *testing.T) {
	dir := initRepo(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	tracked, err := openRepo(t, dir).isTracked(ctx, "a.txt")
	if tracked || !errors.Is(err, ErrTimeout) {
		t.Fatalf("tracked=%v err=%v, want false + timeout", tracked, err)
	}
}

func TestDiffFileFromOutput(t *testing.T) {
	if _, err := diffFileFromOutput(diffJoin("diff --git a/x b/x", "@@ nonsense"), "x"); !errors.Is(err, ErrParseFailed) {
		t.Fatalf("malformed: err = %v, want parse_failed", err)
	}
	two := diffJoin(
		"diff --git a/x.txt b/x.txt", "--- a/x.txt", "+++ b/x.txt", "@@ -1 +1 @@", "-a", "+b",
		"diff --git a/y.txt b/y.txt", "--- a/y.txt", "+++ b/y.txt", "@@ -1 +1 @@", "-c", "+d",
	)
	if _, err := diffFileFromOutput(two, "dir"); !errors.Is(err, ErrValidationFailed) {
		t.Fatalf("multi: err = %v, want validation_failed", err)
	}
	fd, err := diffFileFromOutput([]byte(""), "x.txt")
	if fd != nil || err != nil {
		t.Fatalf("empty output: fd=%v err=%v, want nil,nil", fd, err)
	}
}

func hashOf(t *testing.T, dir, ref string) string {
	t.Helper()
	out, _, err := runGit(context.Background(), dir, "rev-parse", ref)
	if err != nil {
		t.Fatal(err)
	}
	return strings.TrimSpace(string(out))
}

// mergeRepo: main adds+edits, side adds s.txt; HEAD is the merge commit.
func mergeRepo(t *testing.T) (dir, mergeHash, mainHash, rootHash string) {
	t.Helper()
	ctx := context.Background()
	dir = initRepo(t)
	writeFile(t, dir, "base.txt", "b\n")
	commitAll(t, dir, "root")
	rootHash = hashOf(t, dir, "HEAD")
	runGit(ctx, dir, "checkout", "-qb", "side")
	writeFile(t, dir, "s.txt", "side content\nline2\n")
	commitAll(t, dir, "side add")
	runGit(ctx, dir, "checkout", "-q", "main")
	writeFile(t, dir, "base.txt", "B\n")
	commitAll(t, dir, "main edit")
	mainHash = hashOf(t, dir, "HEAD")
	if _, _, err := runGit(ctx, dir, "merge", "side", "-m", "merged"); err != nil {
		t.Fatal(err)
	}
	return dir, hashOf(t, dir, "HEAD"), mainHash, rootHash
}

func diffCommit(t *testing.T, dir, hash, path string) (*FileDiff, error) {
	t.Helper()
	return openRepo(t, dir).DiffCommitFile(context.Background(), hash, path)
}

func TestDiffCommitFileModify(t *testing.T) {
	dir, _, mainHash, _ := mergeRepo(t)
	fd, err := diffCommit(t, dir, mainHash, "base.txt")
	if err != nil {
		t.Fatalf("DiffCommitFile: %v", err)
	}
	if fd.Change != ChangeModified || fd.OldPath != "base.txt" || fd.NewPath != "base.txt" {
		t.Fatalf("got %+v, want modified base.txt", fd)
	}
	lines := fd.Hunks[0].Lines
	if len(lines) != 2 || lines[0] != (DiffLine{Type: DiffLineDel, OldNum: 1, Text: "b"}) ||
		lines[1] != (DiffLine{Type: DiffLineAdd, NewNum: 1, Text: "B"}) {
		t.Fatalf("lines = %+v, want del b / add B", lines)
	}
}

func TestDiffCommitFileMergeFirstParent(t *testing.T) {
	// s.txt came from the side branch: unchanged against HEAD^1, matching
	// against HEAD^2. Plain combined (cc) show would hide it; the
	// first-parent rule must surface the add.
	dir, mergeHash, _, _ := mergeRepo(t)
	fd, err := diffCommit(t, dir, mergeHash, "s.txt")
	if err != nil {
		t.Fatalf("DiffCommitFile: %v", err)
	}
	if fd.Change != ChangeAdded || fd.NewPath != "s.txt" || len(fd.Hunks) != 1 {
		t.Fatalf("got %+v, want added s.txt with one hunk against first parent", fd)
	}
	if fd.Hunks[0].Header != "@@ -0,0 +1,2 @@" {
		t.Fatalf("header = %q, want -0,0 +1,2", fd.Hunks[0].Header)
	}
}

func TestDiffCommitFileRootCommit(t *testing.T) {
	dir, _, _, rootHash := mergeRepo(t)
	fd, err := diffCommit(t, dir, rootHash, "base.txt")
	if err != nil {
		t.Fatalf("DiffCommitFile: %v", err)
	}
	if fd.Change != ChangeAdded || fd.NewPath != "base.txt" || len(fd.Hunks) != 1 {
		t.Fatalf("got %+v, want added base.txt against the empty tree", fd)
	}
}

func TestDiffCommitFileUnchanged(t *testing.T) {
	dir, _, mainHash, _ := mergeRepo(t)
	fd, err := diffCommit(t, dir, mainHash, "s.txt") // not in this commit at all
	if err != nil {
		t.Fatalf("DiffCommitFile: %v", err)
	}
	if fd.Change != ChangeModified || len(fd.Hunks) != 0 || fd.OldPath != "s.txt" || fd.NewPath != "s.txt" {
		t.Fatalf("got %+v, want unchanged s.txt", fd)
	}
}

func TestDiffCommitFileRenameBreaksToPairSides(t *testing.T) {
	// documents the pathspec decision: single-path filtering breaks git's
	// rename pairing, so the new side renders as an add; the rename
	// summary lives in DiffCommitFiles
	ctx := context.Background()
	dir := initRepo(t)
	writeFile(t, dir, "f.txt", "same\ncontent\nacross\nthe rename\nboundary keeps it\nidentical\n")
	commitAll(t, dir, "base")
	runGit(ctx, dir, "mv", "f.txt", "g.txt")
	commitAll(t, dir, "renamed")
	hash := hashOf(t, dir, "HEAD")

	added, err := diffCommit(t, dir, hash, "g.txt")
	if err != nil {
		t.Fatalf("DiffCommitFile new side: %v", err)
	}
	if added.Change != ChangeAdded || added.NewPath != "g.txt" || len(added.Hunks) != 1 {
		t.Fatalf("new side = %+v, want full add of g.txt", added)
	}
	deleted, err := diffCommit(t, dir, hash, "f.txt")
	if err != nil {
		t.Fatalf("DiffCommitFile old side: %v", err)
	}
	if deleted.Change != ChangeDeleted || deleted.OldPath != "f.txt" {
		t.Fatalf("old side = %+v, want full delete of f.txt", deleted)
	}
}

func TestDiffCommitFileValidation(t *testing.T) {
	dir, _, mainHash, _ := mergeRepo(t)
	repo := openRepo(t, dir)

	for _, hash := range []string{"", "-x"} {
		if _, err := repo.DiffCommitFile(context.Background(), hash, "base.txt"); !errors.Is(err, ErrValidationFailed) {
			t.Fatalf("hash %q: err = %v, want validation_failed", hash, err)
		}
	}
	if _, err := repo.DiffCommitFile(context.Background(), mainHash, "../escape"); !errors.Is(err, ErrValidationFailed) {
		t.Fatalf("err = %v, want validation_failed", err)
	}
}

func TestDiffCommitFileUnknownHash(t *testing.T) {
	dir, _, _, _ := mergeRepo(t)
	_, err := diffCommit(t, dir, "0000000000000000000000000000000000000000", "base.txt")
	if !errors.Is(err, ErrCommandFailed) {
		t.Fatalf("err = %v, want command_failed", err)
	}
}

func TestDiffCommitFileCtxKill(t *testing.T) {
	dir, _, mainHash, _ := mergeRepo(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := openRepo(t, dir).DiffCommitFile(ctx, mainHash, "base.txt"); !errors.Is(err, ErrTimeout) {
		t.Fatalf("err = %v, want timeout", err)
	}
}

func statsByPath(t *testing.T, stats []CommitFileStat, hash string) map[string]CommitFileStat {
	t.Helper()
	m := map[string]CommitFileStat{}
	for _, s := range stats {
		m[s.Path] = s
	}
	if len(m) != len(stats) {
		t.Fatalf("duplicate paths in %+v (hash %s)", stats, hash)
	}
	return m
}

func TestDiffCommitFiles(t *testing.T) {
	ctx := context.Background()
	dir := initRepo(t)
	writeFile(t, dir, "a.txt", "one\ntwo\n")
	writeFile(t, dir, "b.txt", "del1\ndel2\n")
	writeFile(t, dir, "k.txt", "keep\n")
	writeFile(t, dir, "r.txt", "rename me content line\nmore lines here\n")
	writeFile(t, dir, "mode.txt", "m\n")
	writeFile(t, dir, "old name.txt", "spaced name\n")
	writeFile(t, dir, "ta\tb.txt", "tabbed name\n")
	writeFileBytes(t, dir, "mod.bin", []byte{0, 1, 2, 3})
	commitAll(t, dir, "base")

	runGit(ctx, dir, "rm", "b.txt")
	runGit(ctx, dir, "mv", "r.txt", "renamed.txt")
	runGit(ctx, dir, "mv", "old name.txt", "new name.txt")
	runGit(ctx, dir, "mv", "ta\tb.txt", "tb.txt")
	writeFile(t, dir, "a.txt", "ONE\ntwo\n")
	writeFile(t, dir, "add.txt", "new1\nnew2\n")
	if err := os.Chmod(filepath.Join(dir, "mode.txt"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFileBytes(t, dir, "mod.bin", []byte{0, 9, 8, 7})
	commitAll(t, dir, "change")

	stats, err := openRepo(t, dir).DiffCommitFiles(ctx, hashOf(t, dir, "HEAD"))
	if err != nil {
		t.Fatalf("DiffCommitFiles: %v", err)
	}
	if len(stats) != 8 {
		t.Fatalf("got %d entries %+v, want 8", len(stats), stats)
	}
	m := statsByPath(t, stats, "HEAD")

	if s := m["b.txt"]; s.Change != ChangeDeleted || s.Added != 0 || s.Deleted != 2 {
		t.Fatalf("b.txt = %+v, want deleted 0/2", s)
	}
	if s := m["a.txt"]; s.Change != ChangeModified || s.Added != 1 || s.Deleted != 1 {
		t.Fatalf("a.txt = %+v, want modified 1/1", s)
	}
	if s := m["add.txt"]; s.Change != ChangeAdded || s.Added != 2 || s.Deleted != 0 {
		t.Fatalf("add.txt = %+v, want added 2/0", s)
	}
	if s := m["renamed.txt"]; s.Change != ChangeRenamed || s.OldPath != "r.txt" ||
		s.Similarity != 100 || s.Added != 0 || s.Deleted != 0 {
		t.Fatalf("renamed.txt = %+v, want R100 r.txt 0/0", s)
	}
	if s := m["new name.txt"]; s.Change != ChangeRenamed || s.OldPath != "old name.txt" {
		t.Fatalf("new name.txt = %+v, want rename of old name.txt", s)
	}
	if s := m["tb.txt"]; s.Change != ChangeRenamed || s.OldPath != "ta\tb.txt" {
		t.Fatalf("tb.txt = %+v, want rename of tabbed path untouched by quoting", s)
	}
	if s := m["mode.txt"]; s.Change != ChangeModified || s.Added != 0 || s.Deleted != 0 {
		t.Fatalf("mode.txt = %+v, want modified 0/0", s)
	}
	if s := m["mod.bin"]; !s.Binary || s.Added != 0 || s.Deleted != 0 {
		t.Fatalf("mod.bin = %+v, want binary counts", s)
	}
}

func TestDiffCommitFilesMergeFirstParent(t *testing.T) {
	dir, mergeHash, _, _ := mergeRepo(t)
	stats, err := openRepo(t, dir).DiffCommitFiles(context.Background(), mergeHash)
	if err != nil {
		t.Fatalf("DiffCommitFiles: %v", err)
	}
	m := statsByPath(t, stats, mergeHash)
	if len(stats) != 1 || m["s.txt"].Change != ChangeAdded {
		t.Fatalf("got %+v, want only s.txt added against first parent", stats)
	}
}

func TestDiffCommitFilesRootAndEmpty(t *testing.T) {
	dir, _, _, rootHash := mergeRepo(t)
	stats, err := openRepo(t, dir).DiffCommitFiles(context.Background(), rootHash)
	if err != nil {
		t.Fatalf("root: %v", err)
	}
	m := statsByPath(t, stats, rootHash)
	if len(stats) != 1 || m["base.txt"].Change != ChangeAdded {
		t.Fatalf("root got %+v, want base.txt added", stats)
	}

	_, _, err = runGit(context.Background(), dir, "commit", "-q", "--allow-empty", "-m", "nothing")
	if err != nil {
		t.Fatal(err)
	}
	stats, err = openRepo(t, dir).DiffCommitFiles(context.Background(), hashOf(t, dir, "HEAD"))
	if err != nil {
		t.Fatalf("empty commit: %v", err)
	}
	if len(stats) != 0 {
		t.Fatalf("got %+v, want no entries", stats)
	}
}

func TestDiffCommitFilesValidation(t *testing.T) {
	dir, _, _, _ := mergeRepo(t)
	repo := openRepo(t, dir)
	for _, hash := range []string{"", "-x"} {
		if _, err := repo.DiffCommitFiles(context.Background(), hash); !errors.Is(err, ErrValidationFailed) {
			t.Fatalf("hash %q: err = %v, want validation_failed", hash, err)
		}
	}
	if _, err := repo.DiffCommitFiles(context.Background(), "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef"); !errors.Is(err, ErrCommandFailed) {
		t.Fatalf("err = %v, want command_failed", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := repo.DiffCommitFiles(ctx, "HEAD"); !errors.Is(err, ErrTimeout) {
		t.Fatalf("err = %v, want timeout", err)
	}
}

func TestParseNameStatusZ(t *testing.T) {
	in := strings.Join([]string{"R100", "old name.txt", "new name.txt", "A", "add.txt"}, "\x00") + "\x00"
	got, err := parseNameStatusZ([]byte(in))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(got) != 2 || got[0].path != "new name.txt" || got[0].oldPath != "old name.txt" ||
		got[0].similarity != 100 || got[0].change != ChangeRenamed ||
		got[1].path != "add.txt" || got[1].change != ChangeAdded {
		t.Fatalf("got %+v", got)
	}
	for _, bad := range []string{"A\x00", "R100\x00only\x00", "\x00x\x00", "R9x\x00a\x00b\x00"} {
		if _, err := parseNameStatusZ([]byte(bad)); !errors.Is(err, ErrParseFailed) {
			t.Fatalf("input %q: err = %v, want parse_failed", bad, err)
		}
	}
}

func TestParseNumstatZ(t *testing.T) {
	in := strings.Join([]string{"1\t2\tpath.txt", "0\t0\t", "old", "new", "-\t-\tbin.dat"}, "\x00") + "\x00"
	got, err := parseNumstatZ([]byte(in))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if p := got["path.txt"]; p.added != 1 || p.deleted != 2 {
		t.Fatalf("path.txt = %+v, want 1/2", p)
	}
	if r := got["new"]; r.added != 0 || r.deleted != 0 {
		t.Fatalf("rename side = %+v", r)
	}
	if b := got["bin.dat"]; !b.binary {
		t.Fatalf("bin.dat = %+v, want binary", b)
	}
	for _, bad := range []string{"x\ty\tz\x00", "0\t0\t\x00old\x00", "no-tabs\x00", "-\t5\tmix\x00", "1\t2\x00", "5\tq\tf\x00"} {
		if _, err := parseNumstatZ([]byte(bad)); !errors.Is(err, ErrParseFailed) {
			t.Fatalf("input %q: err = %v, want parse_failed", bad, err)
		}
	}
}

func TestParseNameStatusZLetters(t *testing.T) {
	in := strings.Join([]string{"C78", "src", "dst", "T", "f.txt", "Q", "x.txt"}, "\x00") + "\x00"
	got, err := parseNameStatusZ([]byte(in))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if got[0].change != ChangeCopied || got[0].similarity != 78 || got[0].oldPath != "src" {
		t.Fatalf("copy = %+v, want C78 src->dst", got[0])
	}
	if got[1].change != ChangeTypeChanged || got[2].change != ChangeUnknown {
		t.Fatalf("letters = %+v, want T then unknown Q", got)
	}
}

func TestParseCommitFileStats(t *testing.T) {
	got, err := parseCommitFileStats(
		[]byte("A\x00n.txt\x00"),
		[]byte("3\t0\tn.txt\x00"),
	)
	if err != nil || len(got) != 1 || got[0].Added != 3 || got[0].Change != ChangeAdded {
		t.Fatalf("got %+v err %v, want one added 3/0", got, err)
	}
	if _, err := parseCommitFileStats([]byte("A\x00"), []byte("")); !errors.Is(err, ErrParseFailed) {
		t.Fatalf("truncated names: err = %v, want parse_failed", err)
	}
	if _, err := parseCommitFileStats([]byte("A\x00n.txt\x00"), []byte("z\t0\tn.txt\x00")); !errors.Is(err, ErrParseFailed) {
		t.Fatalf("bad counts: err = %v, want parse_failed", err)
	}
	if _, err := parseCommitFileStats([]byte("A\x00n.txt\x00"), []byte("1\t0\tOther.txt\x00")); !errors.Is(err, ErrParseFailed) {
		t.Fatalf("mismatched join: err = %v, want parse_failed", err)
	}
}
