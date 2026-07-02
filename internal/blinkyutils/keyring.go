package blinkyutils

import (
	"errors"
	"log/slog"

	"github.com/zalando/go-keyring"
)

// keyringService is the OS-keyring service name that ayato server secrets live
// under, keyed by server name. Keeping the plaintext token out of the on-disk
// server DB is the point of this indirection.
const keyringService = "kamisato-ayato"

// Keyring is the slice of the OS secret store blinkyutils needs. It is an
// interface so tests can inject a fake instead of standing up a real Secret
// Service / Keychain / Credential Manager.
type Keyring interface {
	Get(service, key string) (string, error)
	Set(service, key, secret string) error
	Delete(service, key string) error
}

// errKeyringNotFound is the miss sentinel, distinct from an unavailable backend
// (a headless box with no Secret Service) so a genuine miss falls back to the file
// DB silently while an unavailable backend is logged.
var errKeyringNotFound = keyring.ErrNotFound

// osKeyring is the production backend delegating to go-keyring.
type osKeyring struct{}

func (osKeyring) Get(service, key string) (string, error) { return keyring.Get(service, key) }
func (osKeyring) Set(service, key, secret string) error   { return keyring.Set(service, key, secret) }
func (osKeyring) Delete(service, key string) error        { return keyring.Delete(service, key) }

// secretKeyring is swapped out in tests. It is package-level state on purpose: the
// server commands are thin and share one keyring rather than threading it through.
var secretKeyring Keyring = osKeyring{}

// StoreSecret saves a server credential secret in the OS keyring so it is not
// written to disk in plaintext. It returns true when the keyring accepted it (the
// caller should then leave the file-DB password empty); false means the keyring is
// unavailable — a headless box with no Secret Service — and the caller must keep
// the secret in the file DB as a fallback. An empty secret stores nothing and
// clears any stale keyring entry.
func StoreSecret(server, secret string) bool {
	if secret == "" {
		ForgetSecret(server)
		return false
	}
	if err := secretKeyring.Set(keyringService, server, secret); err != nil {
		slog.Debug("OS keyring unavailable; keeping server secret in the file DB", "server", server, "err", err)
		return false
	}
	return true
}

// LoadSecret returns the effective secret for a server, preferring the OS keyring
// and falling back to fileSecret. The fallback also covers pre-keyring logins that
// still hold their token in the file DB and have not been migrated yet.
func LoadSecret(server, fileSecret string) string {
	secret, err := secretKeyring.Get(keyringService, server)
	if err == nil && secret != "" {
		return secret
	}
	if err != nil && !errors.Is(err, errKeyringNotFound) {
		slog.Debug("OS keyring read failed; falling back to the file DB secret", "server", server, "err", err)
	}
	return fileSecret
}

// ForgetSecret removes a server's keyring-stored secret. A miss is not an error.
func ForgetSecret(server string) {
	if err := secretKeyring.Delete(keyringService, server); err != nil && !errors.Is(err, errKeyringNotFound) {
		slog.Debug("failed to remove server secret from the OS keyring", "server", server, "err", err)
	}
}
