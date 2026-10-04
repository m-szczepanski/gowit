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

// NewApp creates a new App application struct
func NewApp() *App {
	return &App{}
}

// ExampleBind proves the TS <-> Go binding round-trip (issue #4).
// It will be removed once real bound methods land.
func (a *App) ExampleBind() string {
	return "gowit backend is reachable"
}

// startup is called when the app starts. The context is saved
// so we can call runtime methods.
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}

// shutdown is called when the app is closing.
func (a *App) shutdown(ctx context.Context) {
	if a.repo != nil {
		_ = a.repo.Close()
		a.repo = nil
	}
}
