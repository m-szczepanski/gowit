package main

import (
	"context"
	"errors"
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

func TestOpenFolderDelegatesToPicker(t *testing.T) {
	app := NewApp()
	app.startup(context.Background())
	var gotCtx context.Context
	app.pickFolder = func(ctx context.Context) (string, error) {
		gotCtx = ctx
		return "/home/user/project", nil
	}

	path, err := app.OpenFolder()
	if err != nil {
		t.Fatalf("OpenFolder: %v", err)
	}
	if path != "/home/user/project" {
		t.Fatalf("path = %q, want %q", path, "/home/user/project")
	}
	if gotCtx != app.ctx {
		t.Fatal("picker must receive the startup context")
	}
}

func TestOpenFolderCancelReturnsEmpty(t *testing.T) {
	app := NewApp()
	app.startup(context.Background())
	app.pickFolder = func(context.Context) (string, error) { return "", nil }

	path, err := app.OpenFolder()
	if err != nil || path != "" {
		t.Fatalf("OpenFolder on cancel = (%q, %v), want empty", path, err)
	}
}

func TestOpenFolderPropagatesPickerError(t *testing.T) {
	app := NewApp()
	app.startup(context.Background())
	app.pickFolder = func(context.Context) (string, error) {
		return "", errors.New("dialog unavailable")
	}

	if _, err := app.OpenFolder(); err == nil {
		t.Fatal("want picker error to propagate")
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
