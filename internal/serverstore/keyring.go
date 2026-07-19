package serverstore

import (
	"log/slog"

	systemkeyring "github.com/zalando/go-keyring"

	"github.com/Hayao0819/Kamisato/internal/errors"
)

// These identifiers are part of the released storage schema.
const (
	keyringService   = "kamisato-ayato"
	refreshKeySuffix = "\x00refresh"
)

type keyring interface {
	Get(service, key string) (string, error)
	Set(service, key, secret string) error
	Delete(service, key string) error
}

var errKeyringNotFound = systemkeyring.ErrNotFound

type osKeyring struct{}

func (osKeyring) Get(service, key string) (string, error) {
	return systemkeyring.Get(service, key)
}
func (osKeyring) Set(service, key, secret string) error {
	return systemkeyring.Set(service, key, secret)
}
func (osKeyring) Delete(service, key string) error {
	return systemkeyring.Delete(service, key)
}

var secretKeyring keyring = osKeyring{}

func storeAccessToken(server, token string) (bool, error) {
	if token == "" {
		return false, forgetAccessToken(server)
	}
	if err := secretKeyring.Set(keyringService, server, token); err != nil {
		slog.Debug("OS keyring unavailable; keeping access token in server DB", "server", server, "err", err)
		return false, nil
	}
	return true, nil
}

func loadAccessTokenValue(server, fileFallback string) string {
	token, err := loadAccessToken(server, fileFallback)
	if err != nil {
		slog.Error("credential state unavailable; refusing access token", "server", server, "err", err)
		return ""
	}
	return token
}

func loadAccessToken(server, fileFallback string) (string, error) {
	mode, explicit, stateErr := loadCredentialMode(server)
	if stateErr != nil {
		return "", stateErr
	}
	if explicit {
		switch mode.Access {
		case credentialSourceNone:
			return "", nil
		case credentialSourceFile:
			return fileFallback, nil
		case credentialSourceKeyring:
			return loadAccessTokenFromKeyring(server)
		}
	}
	if fileFallback != "" {
		return fileFallback, nil
	}
	return loadAccessTokenFromKeyring(server)
}

func loadAccessTokenFromKeyring(server string) (string, error) {
	token, err := secretKeyring.Get(keyringService, server)
	if err == nil && token != "" {
		return token, nil
	}
	if errors.Is(err, errKeyringNotFound) {
		return "", nil
	}
	return "", errors.WrapErr(err, "read access token from OS keyring")
}

func forgetAccessToken(server string) error {
	if err := secretKeyring.Delete(keyringService, server); err != nil && !errors.Is(err, errKeyringNotFound) {
		return errors.WrapErr(err, "remove access token from OS keyring")
	}
	return nil
}

func storeRefreshToken(server, token string) (bool, error) {
	if token == "" {
		return false, forgetRefreshToken(server)
	}
	if err := secretKeyring.Set(keyringService, server+refreshKeySuffix, token); err != nil {
		slog.Debug("OS keyring unavailable; refresh token not persisted", "server", server, "err", err)
		return false, errors.WrapErr(err, "persist refresh token in OS keyring")
	}
	return true, nil
}

func loadRefreshTokenValue(server string) string {
	token, err := loadRefreshToken(server)
	if err != nil {
		slog.Error("credential state unavailable; refusing refresh token", "server", server, "err", err)
		return ""
	}
	return token
}

func loadRefreshToken(server string) (string, error) {
	mode, explicit, stateErr := loadCredentialMode(server)
	if stateErr != nil {
		return "", stateErr
	}
	if explicit && mode.Refresh == credentialSourceNone {
		return "", nil
	}
	token, err := secretKeyring.Get(keyringService, server+refreshKeySuffix)
	if err != nil {
		if errors.Is(err, errKeyringNotFound) {
			return "", nil
		}
		return "", errors.WrapErr(err, "read refresh token from OS keyring")
	}
	return token, nil
}

func forgetRefreshToken(server string) error {
	if err := secretKeyring.Delete(keyringService, server+refreshKeySuffix); err != nil && !errors.Is(err, errKeyringNotFound) {
		return errors.WrapErr(err, "remove refresh token from OS keyring")
	}
	return nil
}
