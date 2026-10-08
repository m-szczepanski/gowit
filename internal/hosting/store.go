package hosting

import (
	"errors"

	"github.com/zalando/go-keyring"
)

// ErrNoToken marks the absence of any stored/found token; it is not a
// failure of the store itself.
var ErrNoToken = errors.New("no stored token for host")

// SecretStore keeps one token per host at the system boundary. The only
// production implementation is the OS keychain; gowit's own config file
// never receives token material.
type SecretStore interface {
	Get(host string) (string, error)
	Set(host, token string) error
	Remove(host string) error
}

const keyringService = "gowit"

// KeyringStore maps hosts onto OS keychain entries (service "gowit",
// username = host).
type KeyringStore struct{}

func (KeyringStore) Get(host string) (string, error) {
	token, err := keyring.Get(keyringService, host)
	if errors.Is(err, keyring.ErrNotFound) {
		return "", ErrNoToken
	}
	return token, err
}

func (KeyringStore) Set(host, token string) error {
	return keyring.Set(keyringService, host, token)
}

// Remove treats an already-absent entry as success: callers asked for
// "no token stored", which holds either way.
func (KeyringStore) Remove(host string) error {
	err := keyring.Delete(keyringService, host)
	if errors.Is(err, keyring.ErrNotFound) {
		return nil
	}
	return err
}
