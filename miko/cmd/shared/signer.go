package shared

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/Hayao0819/Kamisato/internal/errors"

	"github.com/Hayao0819/Kamisato/internal/auth/apikey"
	"github.com/Hayao0819/Kamisato/internal/conf"
	"github.com/Hayao0819/Kamisato/miko/service"
	"github.com/Hayao0819/Kamisato/miko/signer"
	"github.com/Hayao0819/Kamisato/pkg/pacman/sign"
)

// BuildSigner returns the worker's package signer per config: the local host key
// signing inline (default), or a remote signer that offloads to a dedicated signer
// tier so the worker holds no private key.
func BuildSigner(ctx context.Context, cfg *conf.MikoConfig) (sign.Signer, error) {
	switch cfg.Signing.Mode {
	case "", "disabled":
		return nil, nil
	case "local":
		return BuildHostSigner(ctx, cfg)
	case "remote":
		if cfg.Signing.Remote.URL == "" {
			return nil, errors.NewErr("signing.mode is remote but signing.remote.url is unset")
		}
		apiKey := cfg.Signing.Remote.APIKey
		if env := os.Getenv("MIKO_SIGNING_REMOTE_API_KEY"); env != "" {
			apiKey = env
		}
		if apiKey == "" {
			return nil, errors.NewErr("signing.mode is remote but signing.remote.api_key is unset")
		}
		slog.Info("remote signing enabled", "url", cfg.Signing.Remote.URL)
		return signer.NewRemoteSigner(cfg.Signing.Remote.URL, apiKey)
	default:
		return nil, errors.NewErrf("signing.mode: unknown value %q (want disabled, local or remote)", cfg.Signing.Mode)
	}
}

func ServiceKeyVerifier(cfg *conf.MikoConfig) *apikey.Verifier {
	entries := make([]apikey.Entry, 0, len(cfg.APIKeys)+len(cfg.Auth.APIKeys))
	for index, key := range cfg.APIKeys {
		entries = append(entries, apikey.Entry{
			Name:   fmt.Sprintf("legacy-%d", index+1),
			Key:    key,
			Scopes: []string{apikey.ScopeAll},
		})
	}
	for _, key := range cfg.Auth.APIKeys {
		entries = append(entries, apikey.Entry{Name: key.Name, Principal: key.Principal, Key: key.Key, Scopes: key.Scopes})
	}
	return apikey.NewScopedVerifier(entries)
}

// BuildHostSigner loads (or on first boot generates) the worker host signing key.
// It returns a nil Signer when no key dir is resolvable, leaving signing disabled.
func BuildHostSigner(ctx context.Context, cfg *conf.MikoConfig) (sign.Signer, error) {
	dir := cfg.Signing.KeyDir
	if dir == "" && cfg.DataDir != "" {
		dir = filepath.Join(cfg.DataDir, "keys")
	}
	if dir == "" {
		slog.Warn("host signing disabled: set signing.key_dir or data_dir to enable")
		return nil, nil
	}
	name := cfg.Signing.Name
	if name == "" {
		name = "miko worker"
	}
	email := cfg.Signing.Email
	if email == "" {
		email = "miko@localhost"
	}
	// The passphrase comes only from the environment, never a config file.
	passphrase := os.Getenv("MIKO_SIGNING_PASSPHRASE")
	if passphrase == "" {
		slog.Warn("host signing key is stored unencrypted at rest; set MIKO_SIGNING_PASSPHRASE to encrypt it")
	}
	ks, err := sign.OpenOrCreate(dir, name, email, passphrase)
	if err != nil {
		return nil, err
	}
	slog.Info("host signing enabled", "key_dir", dir, //nolint:gosec // slog escapes structured values; dir is operator-provided config
		"master_fpr", fmt.Sprintf("%X", ks.MasterEntity().PrimaryKey.Fingerprint),
		"worker_fpr", fmt.Sprintf("%X", ks.WorkerEntity().PrimaryKey.Fingerprint))

	if cfg.Ayato.URL != "" {
		registrationCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
		if rerr := service.RegisterWorkerCert(registrationCtx, cfg, ks); rerr != nil {
			return nil, errors.WrapErr(rerr, "register local signing key with ayato")
		}
	}
	return sign.NewHostKeySigner(ks), nil
}
