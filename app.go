package main

import (
	"context"

	"gowit/internal/git"
)

// App struct is a thin bridge between the frontend and internal packages.
// Business logic lives in internal/*, not here (see docs/ARCHITECTURE.md §4).
type App struct {
	ctx  context.Context
	repo *git.Repo
}

func NewApp() *App {
	return &App{}
}

// ExampleBind is a temporary probe proving the TS <-> Go binding round-trip
// (issue #4); delete it once real bound methods land.
func (a *App) ExampleBind() string {
	return "gowit backend is reachable"
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
