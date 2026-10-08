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

// KeyringStore maps hosts onto OS keychain entries (service + username =
// host). The zero value uses the production service; tests set Service to
// stay out of the user's real namespace.
type KeyringStore struct {
	Service string
}

func (s KeyringStore) service() string {
	if s.Service == "" {
		return keyringService
	}
	return s.Service
}

func (s KeyringStore) Get(host string) (string, error) {
	token, err := keyring.Get(s.service(), host)
	if errors.Is(err, keyring.ErrNotFound) {
		return "", ErrNoToken
	}
	return token, err
}

func (s KeyringStore) Set(host, token string) error {
	return keyring.Set(s.service(), host, token)
}

// Remove treats an already-absent entry as success: callers asked for
// "no token stored", which holds either way.
func (s KeyringStore) Remove(host string) error {
	err := keyring.Delete(s.service(), host)
	if errors.Is(err, keyring.ErrNotFound) {
		return nil
	}
	return err
}
