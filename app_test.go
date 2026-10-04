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

	got := app.OpenFolder()
	if got.Path != "/home/user/project" || got.Code != "" {
		t.Fatalf("OpenFolder = %+v, want path set, no code", got)
	}
	if gotCtx != app.ctx {
		t.Fatal("picker must receive the startup context")
	}
}

func TestOpenFolderCancelReturnsEmpty(t *testing.T) {
	app := NewApp()
	app.startup(context.Background())
	app.pickFolder = func(context.Context) (string, error) { return "", nil }

	got := app.OpenFolder()
	if got.Path != "" || got.Code != "" {
		t.Fatalf("OpenFolder on cancel = %+v, want empty result", got)
	}
}

func TestOpenFolderCarriesTypedError(t *testing.T) {
	app := NewApp()
	app.startup(context.Background())
	app.pickFolder = func(context.Context) (string, error) {
		return "", errors.New("dialog unavailable")
	}

	got := app.OpenFolder()
	if got.Code != dialogFailedCode {
		t.Fatalf("Code = %q, want %q", got.Code, dialogFailedCode)
	}
	if got.Message != "dialog unavailable" {
		t.Fatalf("Message = %q, want picker error text", got.Message)
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
