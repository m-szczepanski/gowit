package main

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"gowit/internal/config"
	"gowit/internal/git"
)

// App is a thin bridge between the frontend and internal packages.
// Business logic lives in internal/*, not here (see docs/ARCHITECTURE.md §4).
type App struct {
	ctx        context.Context
	repo       *git.Repo
	pickFolder func(ctx context.Context) (string, error)
	cfg        *config.Store
}

func NewApp() *App {
	return &App{pickFolder: runtimeOpenFolder, cfg: config.NewStore(resolveConfigFile(), time.Now)}
}

// CallResult crosses the Wails boundary as a value: a Go error would
// serialize to a bare string, losing the code (ARCHITECTURE.md §4).
// Empty Code means success.
type CallResult struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

const saveFailedCode = "save_failed"

// GetSettings returns the persisted user settings (defaults when absent).
func (a *App) GetSettings() config.Settings {
	return a.cfg.Settings()
}

func (a *App) SetSettings(settings config.Settings) CallResult {
	if err := a.cfg.SetSettings(settings); err != nil {
		return CallResult{Code: saveFailedCode, Message: err.Error()}
	}
	return CallResult{}
}

func (a *App) GetRecentRepos() []config.RecentRepo {
	return a.cfg.RecentRepos()
}

func (a *App) AddRecentRepo(path string) CallResult {
	if err := a.cfg.AddRecent(path); err != nil {
		return CallResult{Code: saveFailedCode, Message: err.Error()}
	}
	return CallResult{}
}

// resolveConfigFile keeps a usable location when the platform reports no
// config dir (e.g. stripped HOME in CI containers).
func resolveConfigFile() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		dir = os.TempDir()
	}
	return filepath.Join(dir, "gowit", "config.json")
}

// startup keeps the Wails context so runtime methods (EventsEmit) work later.
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}

func (a *App) shutdown(ctx context.Context) {
	if a.repo != nil {
		_ = a.repo.Close()
		_ = a.cfg.AddRecent(a.repo.Path())
		a.repo = nil
	}
}

// FolderDialogResult is the typed outcome of the folder picker
// (ARCHITECTURE.md §4: errors cross the boundary as code+message structs,
// and Wails only ships err.Error() strings, so failures travel as values).
type FolderDialogResult struct {
	Path    string `json:"path"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

const dialogFailedCode = "dialog_failed"

// OpenFolder shows the native folder picker. Path is empty on cancel;
// Code is dialog_failed when the picker itself errored.
func (a *App) OpenFolder() FolderDialogResult {
	path, err := a.pickFolder(a.ctx)
	if err != nil {
		return FolderDialogResult{Code: dialogFailedCode, Message: err.Error()}
	}
	// Recording recents is best effort: an opened folder must not be lost
	// just because the settings file could not be written.
	if path != "" {
		_ = a.cfg.AddRecent(path)
	}
	return FolderDialogResult{Path: path}
}

func runtimeOpenFolder(ctx context.Context) (string, error) {
	return runtime.OpenDirectoryDialog(ctx, runtime.OpenDialogOptions{
		Title: "Open repository folder",
	})
}
