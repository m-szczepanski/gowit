package hosting

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

type fakeStore struct {
	data   map[string]string
	getErr error
}

func (f *fakeStore) Get(host string) (string, error) {
	if f.getErr != nil {
		return "", f.getErr
	}
	v, ok := f.data[host]
	if !ok {
		return "", ErrNoToken
	}
	return v, nil
}

func (f *fakeStore) Set(host, token string) error {
	f.data[host] = token
	return nil
}

func (f *fakeStore) Remove(host string) error {
	delete(f.data, host)
	return nil
}

func newTestTokens(store SecretStore, envName string, env map[string]string, gh string) *Tokens {
	t := &Tokens{
		store:   store,
		envName: func() string { return envName },
		env:     func(name string) string { return env[name] },
	}
	if gh != "" {
		t.gh = func(ctx context.Context) (string, error) { return gh, nil }
	} else {
		t.gh = func(ctx context.Context) (string, error) { return "", errors.New("gh not installed") }
	}
	return t
}

func TestResolveOrder(t *testing.T) {
	ctx := context.Background()
	store := &fakeStore{data: map[string]string{"github.com": "stored"}}

	tok, src, err := newTestTokens(store, "GOWIT_TOKEN", map[string]string{"GOWIT_TOKEN": "fromenv"}, "fromgh").Resolve(ctx, "github.com")
	if tok != "stored" || src != SourceKeyring || err != nil {
		t.Fatalf("stored token lost: %q %q %v", tok, src, err)
	}

	tok, src, err = newTestTokens(&fakeStore{data: map[string]string{}}, "GOWIT_TOKEN", map[string]string{"GOWIT_TOKEN": "fromenv"}, "fromgh").Resolve(ctx, "gitlab.com")
	if tok != "fromenv" || src != SourceEnv || err != nil {
		t.Fatalf("env fallback: %q %q %v", tok, src, err)
	}

	tok, src, err = newTestTokens(&fakeStore{data: map[string]string{}}, "", nil, "fromgh").Resolve(ctx, "gitlab.com")
	if tok != "fromgh" || src != SourceGH || err != nil {
		t.Fatalf("gh fallback: %q %q %v", tok, src, err)
	}

	_, src, err = newTestTokens(&fakeStore{data: map[string]string{}}, "GOWIT_TOKEN", nil, "").Resolve(ctx, "gitlab.com")
	if !errors.Is(err, ErrNoToken) || src != "" {
		t.Fatalf("all empty: src=%q err=%v, want ErrNoToken", src, err)
	}
}

func TestResolvePropagatesStoreFailure(t *testing.T) {
	boom := errors.New("keychain locked")
	_, _, err := newTestTokens(&fakeStore{data: map[string]string{}, getErr: boom}, "E", nil, "").Resolve(context.Background(), "github.com")
	if !errors.Is(err, boom) {
		t.Fatalf("err = %v, want store failure propagated", err)
	}
}

func TestSaveAndClear(t *testing.T) {
	store := &fakeStore{data: map[string]string{}}
	tokens := newTestTokens(store, "", nil, "")

	if err := tokens.Save("github.com", "tok123"); err != nil {
		t.Fatal(err)
	}
	if store.data["github.com"] != "tok123" {
		t.Fatalf("store = %v", store.data)
	}
	if err := tokens.Save("bad host/x", "t"); !errors.Is(err, ErrInvalid) {
		t.Fatalf("host validation: %v", err)
	}
	if err := tokens.Save("github.com", ""); !errors.Is(err, ErrInvalid) {
		t.Fatalf("empty token: %v", err)
	}
	if err := tokens.Clear("github.com"); err != nil {
		t.Fatal(err)
	}
	if _, ok := store.data["github.com"]; ok {
		t.Fatal("clear did not remove")
	}
}

// TestKeyringRoundTrip exercises the real OS keychain. Headless CI has
// no keyring service; the probe skip is the documented CI behavior.
func TestKeyringRoundTrip(t *testing.T) {
	store := KeyringStore{}
	host := "gowit-test-roundtrip.invalid"
	if err := store.Set(host, "secret"); err != nil {
		t.Skipf("no usable OS keyring: %v", err)
	}
	t.Cleanup(func() { store.Remove(host) })

	got, err := store.Get(host)
	if err != nil || got != "secret" {
		t.Fatalf("Get = %q, %v", got, err)
	}
	if err := store.Remove(host); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Get(host); !errors.Is(err, ErrNoToken) {
		t.Fatalf("after Remove: %v, want ErrNoToken", err)
	}
}

// TestNewTokensUsesGhCLI drives the default wiring: NewTokens plugs in
// os.Getenv and the real execGh, so the gh lookup runs through PATH.
func TestNewTokensUsesGhCLI(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX shell shim only")
	}
	dir := t.TempDir()
	shim := filepath.Join(dir, "gh")
	if err := os.WriteFile(shim, []byte("#!/bin/sh\necho shim-token\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))

	tokens := NewTokens(&fakeStore{data: map[string]string{}}, func() string { return "" })
	tok, src, err := tokens.Resolve(context.Background(), "github.com")
	if err != nil || tok != "shim-token" || src != SourceGH {
		t.Fatalf("got %q %q, err %v", tok, src, err)
	}
}

func TestHostValidationRejects(t *testing.T) {
	tokens := newTestTokens(&fakeStore{data: map[string]string{}}, "", nil, "")
	if _, _, err := tokens.Resolve(context.Background(), "bad host"); !errors.Is(err, ErrInvalid) {
		t.Fatalf("Resolve: %v", err)
	}
	if err := tokens.Clear("bad/host"); !errors.Is(err, ErrInvalid) {
		t.Fatalf("Clear: %v", err)
	}
}

func TestExecGhMissingBinaryFails(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	if _, err := execGh(context.Background()); err == nil {
		t.Fatal("want exec failure without gh on PATH")
	}
}
