package main

import (
	"context"
	"testing"
)

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
