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

// OpenFolder shows the native folder picker and returns the chosen path,
// or an empty string when the user cancels.
func (a *App) OpenFolder() (string, error) {
	return a.pickFolder(a.ctx)
}

func runtimeOpenFolder(ctx context.Context) (string, error) {
	return runtime.OpenDirectoryDialog(ctx, runtime.OpenDialogOptions{
		Title: "Open repository folder",
	})
}
