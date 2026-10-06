package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"gowit/internal/config"
	"gowit/internal/git"
	"gowit/internal/watcher"
)

// workTreeWatcher is the watcher seam App drives; *watcher.Watcher
// implements it, tests substitute a controlled stand-in because the real
// one depends on OS event timing.
type workTreeWatcher interface {
	Start(ctx context.Context) error
	Stop() error
}

// App is a thin bridge between the frontend and internal packages.
// Business logic lives in internal/*, not here (see docs/ARCHITECTURE.md §4).
// Wails serves each bound call on its own goroutine, so mu guards the
// shared repo slot.
type App struct {
	mu         sync.Mutex
	ctx        context.Context
	repo       *git.Repo
	pickFolder func(ctx context.Context) (string, error)
	emit       func(ctx context.Context, event string, data ...interface{})
	open       func(path string) (*git.Repo, error)
	watch      func(root string, onChange func()) workTreeWatcher
	watcher    workTreeWatcher
	cfg        *config.Store
}

func NewApp() *App {
	return &App{
		pickFolder: runtimeOpenFolder,
		emit:       runtime.EventsEmit,
		open:       git.Open,
		watch: func(root string, onChange func()) workTreeWatcher {
			return watcher.New(root, onChange, watcher.Options{})
		},
		cfg: config.NewStore(resolveConfigFile(), time.Now),
	}
}

// CallResult crosses the Wails boundary as a value: a Go error would
// serialize to a bare string, losing the code (ARCHITECTURE.md §4).
// Empty Code means success.
type CallResult struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// FolderDialogResult carries the picker outcome: Path is the chosen folder
// (empty on cancel); Code dialog_failed marks a picker error.
type FolderDialogResult struct {
	CallResult
	Path string `json:"path"`
}

// OpenRepositoryResult also carries Path: the resolved work-tree root,
// which differs from the requested path for subdirectories or symlinked
// temp locations - the frontend must key its state on this value.
type OpenRepositoryResult struct {
	CallResult
	Path string `json:"path"`
}

const (
	// statusChangedEvent carries a fresh StatusResponse payload each time the
	// active repo's watcher settles; the UI adopts it instead of refetching.
	statusChangedEvent = "repo:status-changed"

	saveFailedCode = "save_failed"
	openFailedCode = "open_failed"
	callFailedCode = "call_failed"
	noRepoCode     = "no_repo"
)

// StatusResponse is the status read model for the frontend: branch plus
// classified files, with the typed error envelope. Path identifies the repo
// the snapshot belongs to, so event consumers can drop payloads that raced
// a repo switch.
type StatusResponse struct {
	CallResult
	Path   string           `json:"path"`
	Branch git.BranchStatus `json:"branch"`
	Files  []git.FileStatus `json:"files"`
}

// GetStatus returns the parsed working-dir state of the open repository.
// Mutations return the same shape fresh from git, so the UI can replace
// its cache instead of guessing index transitions.
func (a *App) GetStatus() StatusResponse {
	repo := a.currentRepo()
	if repo == nil {
		return StatusResponse{CallResult: CallResult{Code: noRepoCode, Message: "no repository open"}}
	}
	return a.statusOf(repo)
}

func (a *App) statusOf(repo *git.Repo) StatusResponse {
	res, err := repo.Status(a.ctx)
	if err != nil {
		return StatusResponse{CallResult: callResult(err, callFailedCode), Path: repo.Path()}
	}
	return StatusResponse{Path: repo.Path(), Branch: res.Branch, Files: res.Files}
}

// StageFiles adds the given paths to the index.
func (a *App) StageFiles(paths []string) StatusResponse {
	return a.mutate(func(repo *git.Repo) error { return repo.Stage(a.ctx, paths...) })
}

// UnstageFiles resets the index for the given paths, work tree untouched.
func (a *App) UnstageFiles(paths []string) StatusResponse {
	return a.mutate(func(repo *git.Repo) error { return repo.Unstage(a.ctx, paths...) })
}

// StageAll stages every modification, addition and deletion.
func (a *App) StageAll() StatusResponse {
	return a.mutate(func(repo *git.Repo) error { return repo.StageAll(a.ctx) })
}

// UnstageAll resets the whole index back to HEAD.
func (a *App) UnstageAll() StatusResponse {
	return a.mutate(func(repo *git.Repo) error { return repo.UnstageAll(a.ctx) })
}

// DiscardFiles reverts tracked paths and deletes untracked ones; destructive,
// so the UI must confirm before calling.
func (a *App) DiscardFiles(paths []string) StatusResponse {
	return a.mutate(func(repo *git.Repo) error { return repo.DiscardChanges(a.ctx, paths...) })
}

func (a *App) mutate(op func(*git.Repo) error) StatusResponse {
	repo := a.currentRepo()
	if repo == nil {
		return StatusResponse{CallResult: CallResult{Code: noRepoCode, Message: "no repository open"}}
	}
	if err := op(repo); err != nil {
		return StatusResponse{CallResult: callResult(err, callFailedCode)}
	}
	// the echo reports the repo the op actually ran against, even if the
	// slot was swapped while the call was in flight
	return a.statusOf(repo)
}

func (a *App) currentRepo() *git.Repo {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.repo
}

// callResult maps a boundary error to the typed envelope; fallback names
// the adapter family that swallowed a non-GitError.
func callResult(err error, fallback string) CallResult {
	var gitErr *git.GitError
	if errors.As(err, &gitErr) {
		return CallResult{Code: string(gitErr.Code), Message: gitErr.Message}
	}
	return CallResult{Code: fallback, Message: err.Error()}
}

// OpenRepository validates the path with git, binds it as the current repo,
// records it in recents and announces repo:opened. Code carries the git
// failure (not_a_repository, bare_repository, path_missing...) for the UI;
// Path on success is the resolved work-tree root.
func (a *App) OpenRepository(path string) OpenRepositoryResult {
	repo, err := a.open(path)
	if err != nil {
		return OpenRepositoryResult{CallResult: callResult(err, openFailedCode)}
	}

	a.mu.Lock()
	previous := a.repo
	a.repo = repo
	a.mu.Unlock()
	if previous != nil {
		_ = previous.Close()
	}
	// recents are best effort: a failed settings write must not undo an open
	_ = a.cfg.AddRecent(repo.Path())
	a.emit(a.ctx, "repo:opened", repo.Path())
	a.restartWatcher(repo.Path())
	return OpenRepositoryResult{Path: repo.Path()}
}

// restartWatcher moves directory watching onto path, stopping whatever was
// running before. Watching is best effort: a failed Start never invalidates
// the open repository. The onChange closure only spawns emitStatus; calling
// back into locked App methods from the watcher goroutine would invert the
// lock order against a concurrent restart.
func (a *App) restartWatcher(path string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.stopWatcherLocked()
	w := a.watch(path, func() { go a.emitStatus() })
	if err := w.Start(a.ctx); err != nil {
		return
	}
	a.watcher = w
}

// emitStatus runs on a goroutine of its own (see restartWatcher): the
// watcher loop must stay free so Stop can join it, and the detached emit
// keeps onChange from locking a.mu while a restart holds it.
func (a *App) emitStatus() {
	a.emit(a.ctx, statusChangedEvent, a.GetStatus())
}

func (a *App) stopWatcherLocked() {
	if a.watcher != nil {
		_ = a.watcher.Stop()
		a.watcher = nil
	}
}

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
	a.mu.Lock()
	defer a.mu.Unlock()
	a.stopWatcherLocked()
	if a.repo != nil {
		_ = a.repo.Close()
		_ = a.cfg.AddRecent(a.repo.Path())
		a.repo = nil
	}
}

const dialogFailedCode = "dialog_failed"

// OpenFolder shows the native folder picker. Recording and validation are
// the caller's job (OpenRepository) so a rejected folder never lands in
// recents.
func (a *App) OpenFolder() FolderDialogResult {
	path, err := a.pickFolder(a.ctx)
	if err != nil {
		return FolderDialogResult{CallResult: CallResult{Code: dialogFailedCode, Message: err.Error()}}
	}
	return FolderDialogResult{Path: path}
}

func runtimeOpenFolder(ctx context.Context) (string, error) {
	return runtime.OpenDirectoryDialog(ctx, runtime.OpenDialogOptions{
		Title: "Open repository folder",
	})
}
