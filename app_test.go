package main

import (
	"context"
	"testing"

	"gowit/internal/config"
	"gowit/internal/git"
	"gowit/internal/watcher"
)

// No assertions by design: instantiating each type proves the packages
// compile and are importable from the app layer (issue #7 acceptance).
func TestInternalPackagesImportable(t *testing.T) {
	_ = watcher.Watcher{}
	_ = config.Settings{Theme: "dark"}
	_ = config.Profile{RepoPath: "/tmp/repo"}
	_ = git.FileStatus{Path: "main.go", Status: "M."}
	_ = git.Commit{Hash: "a1b2c3d", Subject: "initial commit"}
}

func TestNewApp(t *testing.T) {
	app := NewApp()
	if app == nil {
		t.Fatal("NewApp returned nil")
	}
	if app.repo != nil {
		t.Fatal("new app should not have a repo open")
	}
}

func TestExampleBind(t *testing.T) {
	app := NewApp()
	if got := app.ExampleBind(); got != "gowit backend is reachable" {
		t.Fatalf("ExampleBind returned %q, want %q", got, "gowit backend is reachable")
	}
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
	app := NewApp()
	app.startup(context.Background())
	app.repo = &git.Repo{}

	app.shutdown(context.Background())

	if app.repo != nil {
		t.Fatal("shutdown should close and clear the repo")
	}
}
