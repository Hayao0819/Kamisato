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

// refreshKeySuffix distinguishes a server's refresh-token keyring entry from its
// access-token entry. The refresh token has no file-DB slot, so the keyring is its
// only home; without one it is not persisted and a fresh login re-obtains it.
const refreshKeySuffix = "\x00refresh"

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

// StoreSecret saves a server credential in the OS keyring so it is not written to
// disk in plaintext. It returns true when the keyring accepted it (caller then
// clears the file-DB password) and false when the keyring is unavailable and the
// caller must keep the file-DB fallback. An empty secret clears any stale entry.
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

// StoreRefreshSecret saves a server's refresh token in the OS keyring, returning
// whether the keyring accepted it. An empty token clears any stale entry. Unlike
// the access token there is no file-DB fallback, so false means the refresh token
// is not persisted on this box.
func StoreRefreshSecret(server, secret string) bool {
	if secret == "" {
		ForgetRefreshSecret(server)
		return false
	}
	if err := secretKeyring.Set(keyringService, server+refreshKeySuffix, secret); err != nil {
		slog.Debug("OS keyring unavailable; refresh token not persisted", "server", server, "err", err)
		return false
	}
	return true
}

// LoadRefreshSecret returns a server's stored refresh token, or "" when none is
// stored (or the keyring is unavailable).
func LoadRefreshSecret(server string) string {
	secret, err := secretKeyring.Get(keyringService, server+refreshKeySuffix)
	if err != nil {
		if !errors.Is(err, errKeyringNotFound) {
			slog.Debug("OS keyring read failed for refresh token", "server", server, "err", err)
		}
		return ""
	}
	return secret
}

// ForgetRefreshSecret removes a server's keyring-stored refresh token. A miss is
// not an error.
func ForgetRefreshSecret(server string) {
	if err := secretKeyring.Delete(keyringService, server+refreshKeySuffix); err != nil && !errors.Is(err, errKeyringNotFound) {
		slog.Debug("failed to remove refresh token from the OS keyring", "server", server, "err", err)
	}
}
