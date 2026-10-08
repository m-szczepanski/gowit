package hosting

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"strings"
)

// TokenSource names where a resolved token came from, so the UI can
// explain provenance without ever showing the secret.
type TokenSource string

const (
	SourceKeyring TokenSource = "keyring"
	SourceEnv     TokenSource = "env"
	SourceGH      TokenSource = "gh"
)

// Tokens resolves a host token by preference: keyring first, then the
// env var named in settings (via envName), then the gh CLI. Nothing is
// persisted outside the keyring.
type Tokens struct {
	store   SecretStore
	envName func() string
	env     func(string) string
	gh      func(context.Context) (string, error)
}

func NewTokens(store SecretStore, envName func() string) *Tokens {
	return &Tokens{store: store, envName: envName, env: os.Getenv, gh: execGh}
}

func (t *Tokens) Resolve(ctx context.Context, host string) (string, TokenSource, error) {
	if err := validateHost(host); err != nil {
		return "", "", err
	}
	token, err := t.store.Get(host)
	if err == nil {
		return token, SourceKeyring, nil
	}
	if !errors.Is(err, ErrNoToken) {
		return "", "", err
	}
	if name := t.envName(); name != "" {
		if v := t.env(name); v != "" {
			return v, SourceEnv, nil
		}
	}
	if v, err := t.gh(ctx); err == nil && v != "" {
		return v, SourceGH, nil
	}
	return "", "", ErrNoToken
}

// Save stores the token in the keyring; it is the only path that
// persists, and it persists only into the OS keychain.
func (t *Tokens) Save(host, token string) error {
	if err := validateHost(host); err != nil {
		return err
	}
	if token == "" {
		return &Error{Kind: ErrKindInvalid, Message: "token cannot be empty"}
	}
	return t.store.Set(host, token)
}

func (t *Tokens) Clear(host string) error {
	if err := validateHost(host); err != nil {
		return err
	}
	return t.store.Remove(host)
}

func validateHost(host string) error {
	if host == "" || strings.ContainsAny(host, " /@") {
		return &Error{Kind: ErrKindInvalid, Message: "not a host name: " + host}
	}
	return nil
}

// execGh asks the GitHub CLI for its stored token.
func execGh(ctx context.Context) (string, error) {
	out, err := exec.CommandContext(ctx, "gh", "auth", "token").Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}
