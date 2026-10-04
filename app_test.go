package main

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"gowit/internal/config"
	"gowit/internal/git"
	"gowit/internal/watcher"
)

var testTime = time.Date(2026, 3, 1, 9, 0, 0, 0, time.UTC)

func execGitInit(dir string) ([]byte, error) {
	return exec.Command("git", "init", dir).CombinedOutput()
}

func newTestApp(t *testing.T) (*App, string) {
	t.Helper()
	file := filepath.Join(t.TempDir(), "config.json")
	app := &App{
		cfg: config.NewStore(file, func() time.Time { return testTime }),
	}
	app.startup(context.Background())
	return app, file
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
	_ = git.FileStatus{Path: "main.go", Status: "M."}
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
	app, _ := newTestApp(t)
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
	app, file := newTestApp(t)

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
	app, _ := newTestApp(t)

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

func TestOpenFolderRecordsRecent(t *testing.T) {
	app, _ := newTestApp(t)
	app.pickFolder = func(context.Context) (string, error) { return "/picked/repo", nil }

	got := app.OpenFolder()

	if got.Path != "/picked/repo" || got.Code != "" {
		t.Fatalf("OpenFolder = %+v", got)
	}
	recents := app.GetRecentRepos()
	if len(recents) != 1 || recents[0].Path != "/picked/repo" {
		t.Fatalf("recents = %v, want the opened folder recorded", recents)
	}
}

func TestOpenFolderCancelRecordsNothing(t *testing.T) {
	app, _ := newTestApp(t)
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
	app, _ := newTestApp(t)
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
	fallback := filepath.Join(os.TempDir(), "gowit", "config.json")
	if _, err := os.Stat(fallback); err != nil {
		t.Fatalf("fallback config missing at %s: %v", fallback, err)
	}
	t.Cleanup(func() { _ = os.Remove(fallback) })
}
