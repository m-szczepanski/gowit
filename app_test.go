package main

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"gowit/internal/config"
	"gowit/internal/git"
	"gowit/internal/hosting"
	"gowit/internal/watcher"
)

var testTime = time.Date(2026, 3, 1, 9, 0, 0, 0, time.UTC)

func execGitInit(dir string) ([]byte, error) {
	return exec.Command("git", "init", dir).CombinedOutput()
}

func initRepoForAppTest(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if out, err := execGitInit(dir); err != nil {
		t.Fatalf("git init: %v: %s", err, out)
	}
	// repo-local identity: CI runners have none, and Repo.Commit must not
	// depend on ambient detection (same pin as internal/git's initRepo)
	if out, err := exec.Command("git", "-C", dir, "config", "user.email", "t@t").CombinedOutput(); err != nil {
		t.Fatalf("config email: %v: %s", err, out)
	}
	if out, err := exec.Command("git", "-C", dir, "config", "user.name", "test").CombinedOutput(); err != nil {
		t.Fatalf("config name: %v: %s", err, out)
	}
	return dir
}

type emitted struct {
	event string
	data  []interface{}
}

func newAppBase(t *testing.T) (*App, string) {
	t.Helper()
	file := filepath.Join(t.TempDir(), "config.json")
	app := &App{
		open:  git.Open,
		watch: func(_ string, _ func()) workTreeWatcher { return silentWatcher{} },
		committer: func(ctx context.Context, repo *git.Repo, opts git.CommitOptions) error {
			return repo.Commit(ctx, opts)
		},
		cfg: config.NewStore(file, func() time.Time { return testTime }),
	}
	return app, file
}

func newTestApp(t *testing.T) (*App, string, *[]emitted) {
	t.Helper()
	app, file := newAppBase(t)
	events := &[]emitted{}
	app.emit = func(_ context.Context, event string, data ...interface{}) {
		*events = append(*events, emitted{event, data})
	}
	app.startup(context.Background())
	return app, file, events
}

func TestNewApp(t *testing.T) {
	app := NewApp()
	if app == nil {
		t.Fatal("NewApp returned nil")
	}
	if app.repo != nil {
		t.Fatal("new app should not have a repo open")
	}
	if app.cfg == nil {
		t.Fatal("new app should carry a config store")
	}
}

// No assertions by design: instantiating each type proves the packages
// compile and are importable from the app layer (issue #7 acceptance).
func TestInternalPackagesImportable(t *testing.T) {
	_ = watcher.Watcher{}
	_ = config.Settings{Theme: "dark"}
	_ = config.RecentRepo{Path: "/repo", LastOpened: testTime}
	_ = git.FileStatus{Path: "main.go", XY: ".M"}
	_ = git.Commit{Hash: "a1b2c3d", Subject: "initial commit"}
}

func TestStartupShutdown(t *testing.T) {
	app := NewApp()
	ctx := context.Background()

	app.startup(ctx)
	if app.ctx != ctx {
		t.Fatal("startup did not store the context")
	}

	app.shutdown(ctx)
}

func TestShutdownClosesOpenRepo(t *testing.T) {
	app, _, _ := newTestApp(t)
	dir := t.TempDir()
	if out, err := execGitInit(dir); err != nil {
		t.Fatalf("git init: %v: %s", err, out)
	}
	repo, err := git.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	app.repo = repo

	app.shutdown(context.Background())

	if app.repo != nil {
		t.Fatal("shutdown should close and clear the repo")
	}
	recents := app.GetRecentRepos()
	if len(recents) != 1 || recents[0].Path != repo.Path() {
		t.Fatalf("recents = %v, want the closed repo recorded on shutdown", recents)
	}
}

func TestSettingsRoundTrip(t *testing.T) {
	app, file, _ := newTestApp(t)

	if got := app.GetSettings().Theme; got != config.DefaultTheme {
		t.Fatalf("default Theme = %q, want %q", got, config.DefaultTheme)
	}
	if res := app.SetSettings(config.Settings{Theme: "light"}); res.Code != "" {
		t.Fatalf("SetSettings = %+v, want success", res)
	}

	// a fresh store over the same file proves the write persisted
	fresh := &App{cfg: config.NewStore(file, func() time.Time { return testTime })}
	if got := fresh.GetSettings().Theme; got != "light" {
		t.Fatalf("reopened Theme = %q, want light", got)
	}
}

func TestSetSettingsReportsSaveFailure(t *testing.T) {
	dir := t.TempDir()
	blocked := filepath.Join(dir, "config.json")
	if err := os.Mkdir(blocked, 0o755); err != nil {
		t.Fatal(err)
	}
	app := &App{cfg: config.NewStore(blocked, func() time.Time { return testTime })}

	res := app.SetSettings(config.Settings{Theme: "light"})
	if res.Code != saveFailedCode || res.Message == "" {
		t.Fatalf("SetSettings = %+v, want %s code with message", res, saveFailedCode)
	}
}

func TestRecentReposRoundTrip(t *testing.T) {
	app, _, _ := newTestApp(t)

	if got := app.GetRecentRepos(); len(got) != 0 {
		t.Fatalf("initial recents = %v, want none", got)
	}
	if res := app.AddRecentRepo("/home/dev/alpha"); res.Code != "" {
		t.Fatalf("AddRecentRepo = %+v", res)
	}
	got := app.GetRecentRepos()
	if len(got) != 1 || got[0].Path != "/home/dev/alpha" || !got[0].LastOpened.Equal(testTime) {
		t.Fatalf("recents = %v, want the added repo with injected clock", got)
	}
}

func TestAddRecentRepoReportsSaveFailure(t *testing.T) {
	dir := t.TempDir()
	blocked := filepath.Join(dir, "config.json")
	if err := os.Mkdir(blocked, 0o755); err != nil {
		t.Fatal(err)
	}
	app := &App{cfg: config.NewStore(blocked, func() time.Time { return testTime })}

	res := app.AddRecentRepo("/x")
	if res.Code != saveFailedCode {
		t.Fatalf("AddRecentRepo = %+v, want save_failed", res)
	}
}

func TestOpenFolderRecordsNothing(t *testing.T) {
	app, _, _ := newTestApp(t)
	app.pickFolder = func(context.Context) (string, error) { return "/picked/repo", nil }

	got := app.OpenFolder()

	if got.Path != "/picked/repo" || got.Code != "" {
		t.Fatalf("OpenFolder = %+v", got)
	}
	if len(app.GetRecentRepos()) != 0 {
		t.Fatal("the picker must not record; OpenRepository owns recents")
	}
}

func TestOpenRepositoryBindsEmitsAndRecords(t *testing.T) {
	app, _, events := newTestApp(t)
	dir := initRepoForAppTest(t)
	// git resolves symlinks in the toplevel (macOS /var -> /private/var)
	toplevel, err := filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatal(err)
	}

	res := app.OpenRepository(dir)

	if res.Code != "" {
		t.Fatalf("OpenRepository = %+v", res)
	}
	if res.Path != toplevel {
		t.Fatalf("res.Path = %q, want resolved %q", res.Path, toplevel)
	}
	if app.repo == nil || app.repo.Path() != toplevel {
		t.Fatalf("repo = %v, want bound to %s", app.repo, toplevel)
	}
	if len(*events) != 1 || (*events)[0].event != "repo:opened" || (*events)[0].data[0] != toplevel {
		t.Fatalf("events = %v, want one repo:opened with the path", *events)
	}
	recents := app.GetRecentRepos()
	if len(recents) != 1 || recents[0].Path != toplevel {
		t.Fatalf("recents = %v, want the opened repo", recents)
	}
}

func TestOpenRepositoryReportsGitValidationCodes(t *testing.T) {
	app, _, events := newTestApp(t)

	res := app.OpenRepository(t.TempDir())

	if res.Code != string(git.CodeNotARepository) || res.Message == "" {
		t.Fatalf("OpenRepository(plain dir) = %+v, want not_a_repository with message", res)
	}
	if app.repo != nil {
		t.Fatal("failed open must leave repo unset")
	}
	if len(*events) != 0 {
		t.Fatalf("events = %v, want none on failure", *events)
	}
	if len(app.GetRecentRepos()) != 0 {
		t.Fatal("failed open must not touch recents")
	}
}

func TestOpenRepositoryReplacesPreviousRepo(t *testing.T) {
	app, _, _ := newTestApp(t)
	first := initRepoForAppTest(t)
	second := initRepoForAppTest(t)

	app.OpenRepository(first)
	res := app.OpenRepository(second)

	want, err := filepath.EvalSymlinks(second)
	if err != nil {
		t.Fatal(err)
	}
	if res.Code != "" || app.repo == nil || app.repo.Path() != want {
		t.Fatalf("second open = %+v, repo = %v, want bound to %s", res, app.repo, want)
	}
	if recents := app.GetRecentRepos(); len(recents) != 2 || recents[0].Path != want {
		t.Fatalf("recents = %v, want both opens with newest first", recents)
	}
}

func TestOpenRepositoryReportsUnwrapFailure(t *testing.T) {
	app, _, _ := newTestApp(t)
	app.open = func(string) (*git.Repo, error) { return nil, errors.New("spawn failed") }

	res := app.OpenRepository("/x")

	if res.Code != openFailedCode || res.Message != "spawn failed" {
		t.Fatalf("OpenRepository = %+v, want open_failed fallback", res)
	}
}

func TestOpenFolderCancelRecordsNothing(t *testing.T) {
	app, _, _ := newTestApp(t)
	app.pickFolder = func(context.Context) (string, error) { return "", nil }

	got := app.OpenFolder()

	if got.Path != "" || got.Code != "" {
		t.Fatalf("OpenFolder on cancel = %+v", got)
	}
	if len(app.GetRecentRepos()) != 0 {
		t.Fatal("cancel must not touch recents")
	}
}

func TestOpenFolderCarriesTypedError(t *testing.T) {
	app, _, _ := newTestApp(t)
	app.pickFolder = func(context.Context) (string, error) {
		return "", errors.New("dialog unavailable")
	}

	got := app.OpenFolder()
	if got.Code != dialogFailedCode || got.Message != "dialog unavailable" {
		t.Fatalf("OpenFolder = %+v, want dialog_failed", got)
	}
}

func TestResolveConfigFileUsesPlatformDir(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("HOME", dir)
	t.Setenv("APPDATA", dir)

	base := dir
	switch runtime.GOOS {
	case "darwin":
		base = filepath.Join(dir, "Library", "Application Support")
	case "linux":
		base = dir // XDG_CONFIG_HOME wins
	case "windows":
		base = dir // APPDATA
	}

	app := NewApp()
	if res := app.SetSettings(config.Settings{Theme: "light"}); res.Code != "" {
		t.Fatalf("SetSettings = %+v", res)
	}
	if _, err := os.Stat(filepath.Join(base, "gowit", "config.json")); err != nil {
		t.Fatalf("config not under %s/gowit: %v", base, err)
	}
}

func TestResolveConfigFileFallsBackToTempWhenNoHome(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("HOME", "")
	t.Setenv("APPDATA", "")

	app := NewApp()
	if res := app.SetSettings(config.Settings{Theme: "light"}); res.Code != "" {
		t.Fatalf("SetSettings = %+v", res)
	}
	fallbackDir := filepath.Join(os.TempDir(), "gowit")
	t.Cleanup(func() { _ = os.RemoveAll(fallbackDir) })
	if _, err := os.Stat(filepath.Join(fallbackDir, "config.json")); err != nil {
		t.Fatalf("fallback config missing under %s: %v", fallbackDir, err)
	}
}

// silentWatcher stands in for the OS-bound watcher so #13-style tests never
// start real fsnotify watches.
type silentWatcher struct{}

func (silentWatcher) Start(context.Context) error { return nil }
func (silentWatcher) Stop() error                 { return nil }

type recordingWatcher struct {
	root     string
	startErr error
	started  int
	stopped  int
}

func (r *recordingWatcher) Start(context.Context) error {
	r.started++
	return r.startErr
}

func (r *recordingWatcher) Stop() error {
	r.stopped++
	return nil
}

func newChannelApp(t *testing.T) (*App, chan emitted) {
	t.Helper()
	app, _ := newAppBase(t)
	ch := make(chan emitted, 32)
	app.emit = func(_ context.Context, event string, data ...interface{}) {
		ch <- emitted{event: event, data: data}
	}
	app.startup(context.Background())
	return app, ch
}

func waitEvent(t *testing.T, ch chan emitted, name string) emitted {
	t.Helper()
	deadline := time.After(5 * time.Second)
	for {
		select {
		case ev := <-ch:
			if ev.event == name {
				return ev
			}
			// tolerate the repo:opened preamble; other events are bugs
			if ev.event != repoOpenedEvent {
				t.Fatalf("unexpected event %q while waiting for %q", ev.event, name)
			}
		case <-deadline:
			t.Fatalf("no %q event in time", name)
		}
	}
}

func TestOpenStartsWatcherAndSignalsStatusChanged(t *testing.T) {
	app, ch := newChannelApp(t)
	dir := initRepoWithCommit(t)
	var last *recordingWatcher
	var cb func()
	app.watch = func(root string, onChange func()) workTreeWatcher {
		cb = onChange
		last = &recordingWatcher{root: root}
		return last
	}

	res := app.OpenRepository(dir)
	if res.Code != "" {
		t.Fatalf("open: %+v", res)
	}
	if last.started != 1 {
		t.Fatalf("watcher starts = %d, want 1", last.started)
	}
	want, _ := filepath.EvalSymlinks(dir)
	if last.root != want {
		t.Fatalf("watch root = %q, want resolved %q", last.root, want)
	}
	if cb == nil {
		t.Fatal("no onChange installed")
	}

	cb()
	ev := waitEvent(t, ch, statusChangedEvent)
	if len(ev.data) != 1 {
		t.Fatalf("payload count = %d, want the StatusResponse", len(ev.data))
	}
	snap, ok := ev.data[0].(StatusResponse)
	if !ok {
		t.Fatalf("payload type %T, want StatusResponse", ev.data[0])
	}
	if snap.Code != "" || snap.Path != want || snap.Branch.Head == "" || len(snap.Files) != 0 {
		t.Fatalf("payload = %+v, want clean snapshot of %s", snap, want)
	}
}

func TestWatcherSignalEmitsFreshPayloadEndToEnd(t *testing.T) {
	app, ch := newChannelApp(t)
	app.watch = func(root string, onChange func()) workTreeWatcher {
		return watcher.New(root, onChange, watcher.Options{Debounce: 20 * time.Millisecond})
	}
	repoA := initRepoWithCommit(t)
	repoB := initRepoWithCommit(t)
	resolvedA, _ := filepath.EvalSymlinks(repoA)
	resolvedB, _ := filepath.EvalSymlinks(repoB)

	if res := app.OpenRepository(repoA); res.Code != "" {
		t.Fatalf("open A: %+v", res)
	}
	writeFileForAppTest(t, repoA, "fresh.txt", "x\n")
	ev := waitEvent(t, ch, statusChangedEvent)
	snap := ev.data[0].(StatusResponse)
	if snap.Path != resolvedA || len(snap.Files) == 0 || snap.Files[0].Path != "fresh.txt" {
		t.Fatalf("payload after touching A = %+v", snap)
	}

	if res := app.OpenRepository(repoB); res.Code != "" {
		t.Fatalf("open B: %+v", res)
	}
	drainQuietApp(ch)

	writeFileForAppTest(t, repoB, "b-only.txt", "y\n")
	ev = waitEvent(t, ch, statusChangedEvent)
	snap = ev.data[0].(StatusResponse)
	if snap.Path != resolvedB || snap.Files[0].Path != "b-only.txt" {
		t.Fatalf("payload after touching B = %+v", snap)
	}

	// A's watcher must be gone: whatever still arrives after touching A has
	// to be a duplicate snapshot of B, never an A payload
	writeFileForAppTest(t, repoA, "late.txt", "z\n")
	deadline := time.After(1500 * time.Millisecond)
	for {
		select {
		case ev2 := <-ch:
			if ev2.event != statusChangedEvent {
				continue
			}
			s2 := ev2.data[0].(StatusResponse)
			if s2.Path == resolvedA {
				t.Fatalf("stale watcher fired with A payload: %+v", s2)
			}
		case <-deadline:
			app.shutdown(context.Background())
			return
		}
	}
}

func drainQuietApp(ch chan emitted) {
	for {
		select {
		case <-ch:
		case <-time.After(300 * time.Millisecond):
			return
		}
	}
}

func TestReopenStopsPreviousWatcher(t *testing.T) {
	app, _, _ := newTestApp(t)
	first := initRepoForAppTest(t)
	second := initRepoForAppTest(t)
	var watchers []*recordingWatcher
	app.watch = func(root string, onChange func()) workTreeWatcher {
		w := &recordingWatcher{root: root}
		watchers = append(watchers, w)
		return w
	}

	app.OpenRepository(first)
	app.OpenRepository(second)
	if watchers[0].stopped != 1 || watchers[1].started != 1 {
		t.Fatalf("swap did not replace watcher: %v %+v", *watchers[0], *watchers[1])
	}

	app.shutdown(context.Background())
	if watchers[1].stopped != 1 {
		t.Fatalf("shutdown did not stop active watcher: %+v", *watchers[1])
	}
}

func TestWatcherStartFailureKeepsOpenValid(t *testing.T) {
	app, _, _ := newTestApp(t)
	dir := initRepoForAppTest(t)
	app.watch = func(root string, onChange func()) workTreeWatcher {
		return &recordingWatcher{startErr: errors.New("no inotify left")}
	}

	if res := app.OpenRepository(dir); res.Code != "" {
		t.Fatalf("open should survive watcher failure, got %+v", res)
	}
	app.mu.Lock()
	running := app.watcher
	app.mu.Unlock()
	if running != nil {
		t.Fatal("failed watcher must not be stored")
	}
}

func TestQueueStatusCollapsesDuplicates(t *testing.T) {
	app, _ := newAppBase(t)
	app.signals = make(chan struct{}, 1)
	app.queueStatus()
	app.queueStatus()
	if len(app.signals) != 1 {
		t.Fatalf("queued = %d, want one coalesced signal", len(app.signals))
	}
	<-app.signals
	app.queueStatus()
	if len(app.signals) != 1 {
		t.Fatal("signal after drain was dropped")
	}
}

func TestShutdownStopsStatusWorker(t *testing.T) {
	app, ch := newChannelApp(t)
	dir := initRepoWithCommit(t)
	var cb func()
	app.watch = func(_ string, onChange func()) workTreeWatcher {
		cb = onChange
		return silentWatcher{}
	}
	app.OpenRepository(dir)
	waitEvent(t, ch, repoOpenedEvent)

	app.shutdown(context.Background())
	cb()
	select {
	case ev := <-ch:
		t.Fatalf("worker still emitting after shutdown: %v", ev)
	case <-time.After(500 * time.Millisecond):
	}
}

func TestNewAppDefaultWatchSeamConstructsWatcher(t *testing.T) {
	app := NewApp()
	w := app.watch(t.TempDir(), func() {})
	if w == nil {
		t.Fatal("default watch returned nil")
	}
	if err := w.Stop(); err != nil {
		t.Fatalf("Stop before Start: %v", err)
	}
}

func TestGetStatusRequiresOpenRepo(t *testing.T) {
	app, _, _ := newTestApp(t)
	res := app.GetStatus()
	if res.Code != "no_repo" || res.Message == "" {
		t.Fatalf("GetStatus closed = %+v, want no_repo code", res)
	}
	if res.Files != nil {
		t.Fatalf("files = %v, want nil", res.Files)
	}
}

func TestGetStatusReturnsParsedState(t *testing.T) {
	app, _, _ := newTestApp(t)
	dir := initRepoWithCommit(t)
	if res := app.OpenRepository(dir); res.Code != "" {
		t.Fatalf("open: %+v", res)
	}
	writeFileForAppTest(t, dir, "a.txt", "edited\n")

	res := app.GetStatus()
	if res.Code != "" {
		t.Fatalf("GetStatus: %+v", res)
	}
	wantOid := gitRunIn(t, dir, "rev-parse", "HEAD")
	if res.Branch.Oid != wantOid || res.Branch.Head == "" {
		t.Fatalf("branch = %+v, want at %s", res.Branch, wantOid)
	}
	if len(res.Files) != 1 {
		t.Fatalf("files = %+v, want one", res.Files)
	}
	f := res.Files[0]
	if f.Path != "a.txt" || f.XY != ".M" || !f.Unstaged || f.Staged || f.Change != git.ChangeModified {
		t.Fatalf("file = %+v, want unstaged modified a.txt", f)
	}
}

func TestStageAndUnstageAdapters(t *testing.T) {
	app, _, _ := newTestApp(t)
	dir := initRepoWithCommit(t)
	app.OpenRepository(dir)
	writeFileForAppTest(t, dir, "a.txt", "edited\n")

	if res := app.StageFiles([]string{"a.txt"}); res.Code != "" {
		t.Fatalf("StageFiles: %+v", res)
	}
	f := app.GetStatus().Files[0]
	if !f.Staged || f.Unstaged || f.Change != git.ChangeModified {
		t.Fatalf("after stage: %+v", f)
	}

	if res := app.UnstageFiles([]string{"a.txt"}); res.Code != "" {
		t.Fatalf("UnstageFiles: %+v", res)
	}
	if f := app.GetStatus().Files[0]; f.Staged || !f.Unstaged {
		t.Fatalf("after unstage: %+v", f)
	}

	writeFileForAppTest(t, dir, "a.txt", "edited\n")
	if res := app.StageAll(); res.Code != "" {
		t.Fatalf("StageAll: %+v", res)
	}
	if f := app.GetStatus().Files[0]; !f.Staged {
		t.Fatalf("after StageAll: %+v", f)
	}
	if res := app.UnstageAll(); res.Code != "" {
		t.Fatalf("UnstageAll: %+v", res)
	}
	if f := app.GetStatus().Files[0]; f.Staged {
		t.Fatalf("after UnstageAll: %+v", f)
	}
}

func TestStagingAdaptersSurfaceGitAndRepoErrors(t *testing.T) {
	app, _, _ := newTestApp(t)
	if res := app.StageFiles([]string{"a.txt"}); res.Code != "no_repo" {
		t.Fatalf("closed StageFiles: %+v", res)
	}
	if res := app.UnstageAll(); res.Code != "no_repo" {
		t.Fatalf("closed UnstageAll: %+v", res)
	}

	dir := initRepoWithCommit(t)
	app.OpenRepository(dir)
	res := app.StageFiles([]string{"nosuch.txt"})
	if res.Code != string(git.CodeCommandFailed) || !strings.Contains(res.Message, "nosuch.txt") {
		t.Fatalf("bad path StageFiles = %+v, want command_failed naming the path", res)
	}
	if res := app.StageFiles([]string{}); res.Code != string(git.CodeValidationFailed) {
		t.Fatalf("empty paths = %+v, want validation_failed", res)
	}
}

func initRepoWithCommit(t *testing.T) string {
	t.Helper()
	dir := initRepoForAppTest(t)
	writeFileForAppTest(t, dir, "a.txt", "base\n")
	if out, err := exec.Command("git", "-C", dir, "add", "a.txt").CombinedOutput(); err != nil {
		t.Fatalf("add: %v: %s", err, out)
	}
	if out, err := exec.Command("git", "-C", dir, "commit", "-qm", "base").CombinedOutput(); err != nil {
		t.Fatalf("commit: %v: %s", err, out)
	}
	return dir
}

func writeFileForAppTest(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func gitRunIn(t *testing.T, dir string, args ...string) string {
	t.Helper()
	out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v: %s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func TestGetStatusAfterRepoVanished(t *testing.T) {
	app, _, _ := newTestApp(t)
	dir := initRepoWithCommit(t)
	app.OpenRepository(dir)
	if err := os.RemoveAll(dir); err != nil {
		t.Fatal(err)
	}
	res := app.GetStatus()
	if res.Code != string(git.CodeCommandFailed) {
		t.Fatalf("GetStatus vanished = %+v, want command_failed", res)
	}
}

func TestCallResultFallsBackForNonGitErrors(t *testing.T) {
	res := callResult(errors.New("boom"), callFailedCode)
	if res.Code != callFailedCode || res.Message != "boom" {
		t.Fatalf("callResult = %+v, want call_failed passthrough", res)
	}
	if open := callResult(errors.New("spawn"), openFailedCode); open.Code != openFailedCode {
		t.Fatalf("open fallback = %+v", open)
	}
}

func TestDiscardFilesAdapter(t *testing.T) {
	app, _, _ := newTestApp(t)
	if res := app.DiscardFiles([]string{"a.txt"}); res.Code != noRepoCode {
		t.Fatalf("closed = %+v, want no_repo", res)
	}

	dir := initRepoWithCommit(t)
	app.OpenRepository(dir)
	writeFileForAppTest(t, dir, "a.txt", "ruined\n")
	writeFileForAppTest(t, dir, "scratch.txt", "temp\n")

	res := app.DiscardFiles([]string{"a.txt", "scratch.txt"})
	if res.Code != "" {
		t.Fatalf("DiscardFiles = %+v", res)
	}
	if len(res.Files) != 0 {
		t.Fatalf("after discard = %+v, want clean", res.Files)
	}

	if res := app.DiscardFiles([]string{"nosuch.txt"}); res.Code != string(git.CodeCommandFailed) {
		t.Fatalf("unknown path = %+v, want command_failed", res)
	}
}

func TestCommitAdapter(t *testing.T) {
	app, ch := newChannelApp(t)
	dir := initRepoForAppTest(t)
	if res := app.Commit("nothing open", false); res.Code != noRepoCode {
		t.Fatalf("closed repo = %+v, want no_repo", res)
	}
	opened := app.OpenRepository(dir)
	if opened.Code != "" {
		t.Fatalf("open: %+v", opened)
	}
	writeFileForAppTest(t, dir, "a.txt", "content\n")
	gitRunIn(t, dir, "add", "a.txt")

	if res := app.Commit("", false); res.Code != string(git.CodeValidationFailed) {
		t.Fatalf("empty message = %+v, want validation_failed", res)
	}
	if res := app.Commit("add a", false); res.Code != "" {
		t.Fatalf("Commit = %+v", res)
	}
	if got := gitRunIn(t, dir, "log", "-1", "--format=%s"); got != "add a" {
		t.Fatalf("subject = %q", got)
	}
	// the success nudge reaches the UI as a fresh repo:status-changed;
	// snap.Path is the Cleaned work-tree root, so compare against the
	// app's own resolved path: raw git output uses forward slashes and
	// 8.3 short names on Windows and never matches there
	sawStatus := false
	deadline := time.After(3 * time.Second)
	for !sawStatus {
		select {
		case ev := <-ch:
			if ev.event == statusChangedEvent {
				snap := ev.data[0].(StatusResponse)
				if snap.Path == opened.Path && len(snap.Files) == 0 {
					sawStatus = true
				}
			}
		case <-deadline:
			t.Fatal("no status event after successful commit")
		}
	}

	// amend rewrites HEAD with --no-edit semantics
	writeFileForAppTest(t, dir, "b.txt", "more\n")
	gitRunIn(t, dir, "add", "b.txt")
	if res := app.Commit("", true); res.Code != "" {
		t.Fatalf("amend = %+v", res)
	}
	if got := gitRunIn(t, dir, "log", "-1", "--format=%s"); got != "add a" {
		t.Fatalf("amended subject = %q, want message kept", got)
	}
	files := gitRunIn(t, dir, "show", "--name-only", "--format=", "HEAD")
	if !strings.Contains(files, "b.txt") {
		t.Fatalf("amended tree = %q", files)
	}

	if res := app.Commit("nothing staged", false); res.Code != string(git.CodeNothingToCommit) {
		t.Fatalf("clean commit = %+v, want nothing_to_commit", res)
	}
}

func TestCommitAdapterMapsTypedAndPlainErrors(t *testing.T) {
	app, _ := newChannelApp(t)
	dir := initRepoForAppTest(t)
	app.OpenRepository(dir)

	app.committer = func(_ context.Context, _ *git.Repo, opts git.CommitOptions) error {
		return &git.GitError{Code: git.CodeCommitRejected, Message: "hook said no", ExitCode: 1}
	}
	res := app.Commit("x", false)
	if res.Code != string(git.CodeCommitRejected) || !strings.Contains(res.Message, "hook said no") {
		t.Fatalf("rejected = %+v", res)
	}

	app.committer = func(_ context.Context, _ *git.Repo, _ git.CommitOptions) error {
		return errors.New("boom")
	}
	res = app.Commit("x", false)
	if res.Code != callFailedCode || res.Message != "boom" {
		t.Fatalf("plain error = %+v, want call_failed passthrough", res)
	}
}

func TestNewAppDefaultCommitSeamUsesRealCommit(t *testing.T) {
	app := NewApp()
	dir := initRepoWithCommit(t)
	writeFileForAppTest(t, dir, "second.txt", "x\n")
	gitRunIn(t, dir, "add", "second.txt")
	repo, err := app.open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := app.committer(context.Background(), repo, git.CommitOptions{Message: "real"}); err != nil {
		t.Fatalf("default committer: %v", err)
	}
	if got := gitRunIn(t, dir, "log", "-1", "--format=%s"); got != "real" {
		t.Fatalf("subject = %q", got)
	}
}

type fakeSecrets struct {
	data map[string]string
	err  error
}

func (f *fakeSecrets) Get(host string) (string, error) {
	if f.err != nil {
		return "", f.err
	}
	v, ok := f.data[host]
	if !ok {
		return "", hosting.ErrNoToken
	}
	return v, nil
}

func (f *fakeSecrets) Set(host, token string) error {
	f.data[host] = token
	return nil
}

func (f *fakeSecrets) Remove(host string) error {
	if f.err != nil {
		return f.err
	}
	delete(f.data, host)
	return nil
}

func tokensApp(t *testing.T) (*App, *fakeSecrets) {
	t.Helper()
	app, _, _ := newTestApp(t)
	fake := &fakeSecrets{data: map[string]string{}}
	app.hostTokens = hosting.NewTokens(fake,
		func() string { return app.cfg.Settings().HostTokenEnv },
		func(ctx context.Context) (string, error) { return "", errors.New("gh absent") },
	)
	return app, fake
}

func TestSaveClearHostToken(t *testing.T) {
	app, fake := tokensApp(t)
	if res := app.SaveHostToken("github.com", "tok-123"); res.Code != "" {
		t.Fatalf("save = %+v", res)
	}
	if fake.data["github.com"] != "tok-123" {
		t.Fatalf("fake = %v", fake.data)
	}
	status := app.GetHostToken("github.com")
	if !status.Found || status.Source != "keyring" {
		t.Fatalf("status = %+v, want found via keyring", status)
	}
	if res := app.ClearHostToken("github.com"); res.Code != "" {
		t.Fatalf("clear = %+v", res)
	}
	fake.err = errors.New("keychain locked")
	if res := app.ClearHostToken("github.com"); res.Code != callFailedCode {
		t.Fatalf("clear with store failure = %+v, want call_failed", res)
	}
	fake.err = nil
	if status := app.GetHostToken("github.com"); status.Found || status.Code != "" {
		t.Fatalf("after clear = %+v, want not found without error", status)
	}
}

func TestSaveHostTokenRejectsBadInput(t *testing.T) {
	app, fake := tokensApp(t)
	if res := app.SaveHostToken("github.com", ""); res.Code != "invalid" {
		t.Fatalf("empty token = %+v, want invalid", res)
	}
	if res := app.SaveHostToken("not a host", "t"); res.Code != "invalid" {
		t.Fatalf("bad host = %+v, want invalid", res)
	}
	if len(fake.data) != 0 {
		t.Fatalf("rejected saves must not store: %v", fake.data)
	}
}

func TestGetHostTokenEnvFallback(t *testing.T) {
	app, _ := tokensApp(t)
	t.Setenv("GOWIT_TEST_TOKEN", "env-token")
	if err := app.cfg.SetSettings(config.Settings{Theme: config.DefaultTheme, HostTokenEnv: "GOWIT_TEST_TOKEN"}); err != nil {
		t.Fatal(err)
	}
	status := app.GetHostToken("gitlab.com")
	if !status.Found || status.Source != "env" {
		t.Fatalf("status = %+v, want env source", status)
	}
}

func TestGetHostTokenStoreFailure(t *testing.T) {
	app, fake := tokensApp(t)
	fake.err = errors.New("keychain locked")
	status := app.GetHostToken("github.com")
	if status.Found || status.Code != callFailedCode {
		t.Fatalf("status = %+v, want call_failed passthrough", status)
	}
}

func subAppFixture(t *testing.T) *App {
	t.Helper()
	app, _, _ := newTestApp(t)
	tmp := t.TempDir()
	gitRunIn(t, tmp, "init", "-q", "--bare", "-b", "main", "seed.git")
	gitRunIn(t, tmp, "clone", "-q", "seed.git", "work")
	work := filepath.Join(tmp, "work")
	if out, err := exec.Command("git", "-C", work, "config", "user.email", "t@t").CombinedOutput(); err != nil {
		t.Fatal(err, string(out))
	}
	if out, err := exec.Command("git", "-C", work, "config", "user.name", "t").CombinedOutput(); err != nil {
		t.Fatal(err, string(out))
	}
	if err := os.WriteFile(filepath.Join(work, "lib.txt"), []byte("lib\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitRunIn(t, work, "add", ".")
	gitRunIn(t, work, "commit", "-qm", "lib")
	gitRunIn(t, work, "push", "-q", "origin", "main")

	super := filepath.Join(tmp, "super")
	gitRunIn(t, tmp, "init", "-q", "-b", "main", "super")
	if out, err := exec.Command("git", "-C", super, "config", "user.email", "t@t").CombinedOutput(); err != nil {
		t.Fatal(err, string(out))
	}
	if out, err := exec.Command("git", "-C", super, "config", "user.name", "t").CombinedOutput(); err != nil {
		t.Fatal(err, string(out))
	}
	if err := os.WriteFile(filepath.Join(super, "README"), []byte("top\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitRunIn(t, super, "add", ".")
	gitRunIn(t, super, "commit", "-qm", "top")
	gitRunIn(t, super, "-c", "protocol.file.allow=always", "submodule", "add", "-q", "../work", "sub1")
	gitRunIn(t, super, "commit", "-qm", "add sub1")
	if res := app.OpenRepository(super); res.Code != "" {
		t.Fatalf("open super: %+v", res)
	}
	return app
}

func TestAppGetSubmodules(t *testing.T) {
	app := subAppFixture(t)
	res := app.GetSubmodules()
	if res.Code != "" || len(res.Submodules) != 1 {
		t.Fatalf("res = %+v", res)
	}
	s := res.Submodules[0]
	if s.Path != "sub1" || s.State != "ok" {
		t.Fatalf("sub = %+v", s)
	}
}

func TestAppSubmoduleInitUpdateAndOpen(t *testing.T) {
	app := subAppFixture(t)
	if res := app.SubmoduleDeinit("sub1"); res.Code != "" {
		t.Fatalf("deinit: %+v", res)
	}
	if res := app.GetSubmodules(); len(res.Submodules) != 1 || res.Submodules[0].State != "uninitialized" {
		t.Fatalf("after deinit: %+v", res.Submodules)
	}
	if res := app.SubmoduleUpdate("sub1"); res.Code != "" {
		t.Fatalf("update one: %+v", res)
	}
	if res := app.SubmoduleInitUpdate(true); res.Code != "" {
		t.Fatalf("init update all: %+v", res)
	}
	opened := app.OpenSubmodule("sub1")
	if opened.Code != "" {
		t.Fatalf("deep link: %+v", opened)
	}
	if !strings.HasSuffix(opened.Path, "sub1") {
		t.Fatalf("path = %q", opened.Path)
	}
	inside := app.GetSubmodules()
	if inside.Code != "" || len(inside.Submodules) != 0 {
		t.Fatalf("inside submodule, no submodules expected: %+v", inside)
	}
}

func TestAppSubmoduleAddAndRemove(t *testing.T) {
	app := subAppFixture(t)
	t.Setenv("GIT_CONFIG_COUNT", "1")
	t.Setenv("GIT_CONFIG_KEY_0", "protocol.file.allow")
	t.Setenv("GIT_CONFIG_VALUE_0", "always")

	tmp := t.TempDir()
	deep := filepath.Join(tmp, "deep2")
	gitRunIn(t, tmp, "init", "-q", "-b", "main", "deep2")
	gitRunIn(t, deep, "config", "user.email", "t@t")
	gitRunIn(t, deep, "config", "user.name", "t")
	if err := os.WriteFile(filepath.Join(deep, "d.txt"), []byte("d\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitRunIn(t, deep, "add", ".")
	gitRunIn(t, deep, "commit", "-qm", "d")

	if res := app.SubmoduleAdd(deep, "sub2"); res.Code != "" {
		t.Fatalf("add: %+v", res)
	}
	list := app.GetSubmodules()
	if len(list.Submodules) != 2 {
		t.Fatalf("after add: %+v", list.Submodules)
	}
	// add only stages; git refuses to rm a gitlink whose .gitmodules
	// change is still uncommitted, matching the real GUI flow (add,
	// review, commit, later remove)
	if res := app.Commit("add sub2", false); res.Code != "" {
		t.Fatalf("commit the staged add: %+v", res)
	}
	if res := app.SubmoduleRemove("sub2"); res.Code != "" {
		t.Fatalf("remove: %+v", res)
	}
	if list := app.GetSubmodules(); len(list.Submodules) != 1 {
		t.Fatalf("after remove: %+v", list.Submodules)
	}
}

func TestAppSubmoduleGuardsAndNoRepo(t *testing.T) {
	app := subAppFixture(t)
	if res := app.OpenSubmodule("README"); res.Code != "validation_failed" {
		t.Fatalf("unregistered deep link: %+v", res)
	}
	if res := app.SubmoduleRemove("nope"); res.Code != "validation_failed" {
		t.Fatalf("remove unregistered: %+v", res)
	}
	if res := app.GetSubmodules(); res.Path == "" {
		t.Fatal("response must echo repo path")
	}

	noRepo := &App{}
	if res := noRepo.GetSubmodules(); res.Code != noRepoCode {
		t.Fatalf("no repo: %+v", res)
	}
	if res := noRepo.SubmoduleInitUpdate(false); res.Code != noRepoCode {
		t.Fatalf("no repo mutate: %+v", res)
	}
	if res := noRepo.OpenSubmodule("x"); res.Code != noRepoCode {
		t.Fatalf("no repo open: %+v", res)
	}
}
