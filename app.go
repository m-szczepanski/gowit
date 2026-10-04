package main

import (
	"context"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"gowit/internal/git"
)

// App is a thin bridge between the frontend and internal packages.
// Business logic lives in internal/*, not here (see docs/ARCHITECTURE.md §4).
type App struct {
	ctx        context.Context
	repo       *git.Repo
	pickFolder func(ctx context.Context) (string, error)
}

func NewApp() *App {
	return &App{pickFolder: runtimeOpenFolder}
}

// startup keeps the Wails context so runtime methods (EventsEmit) work later.
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}

func (a *App) shutdown(ctx context.Context) {
	if a.repo != nil {
		_ = a.repo.Close()
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
	return FolderDialogResult{Path: path}
}

func runtimeOpenFolder(ctx context.Context) (string, error) {
	return runtime.OpenDirectoryDialog(ctx, runtime.OpenDialogOptions{
		Title: "Open repository folder",
	})
}
