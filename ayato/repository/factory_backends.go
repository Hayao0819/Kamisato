package repository

import (
	"log/slog"
	"os"

	"github.com/Hayao0819/Kamisato/ayato/auth/secretbox"
	"github.com/Hayao0819/Kamisato/ayato/blob"
	"github.com/Hayao0819/Kamisato/ayato/blob/localfs"
	"github.com/Hayao0819/Kamisato/ayato/blob/s3"
	"github.com/Hayao0819/Kamisato/ayato/domain"
	"github.com/Hayao0819/Kamisato/ayato/repository/kv"
	"github.com/Hayao0819/Kamisato/ayato/repository/kv/badgerkv"
	"github.com/Hayao0819/Kamisato/ayato/repository/kv/cfkv"
	"github.com/Hayao0819/Kamisato/ayato/repository/kv/securekv"
	"github.com/Hayao0819/Kamisato/ayato/repository/kv/sqlkv"
	"github.com/Hayao0819/Kamisato/internal/errors"
)

var defaultEncryptedNamespaces = []string{kv.AdminAllowlist}

type initializedStores struct {
	catalog *domain.RepositoryCatalog
	kv      kv.Store
	binary  blob.Store
}

// initializeStores is the shared composition path for the runtime and migration
// commands. It owns cleanup until every component has initialized successfully.
func initializeStores(settings Settings) (stores initializedStores, err error) {
	catalog := settings.Catalog
	if catalog == nil {
		catalog, _ = domain.NewRepositoryCatalog(nil, nil)
	}

	kvStore, err := initKVStore(settings.KV)
	if err != nil {
		slog.Error("Failed to create key-value store", "error", err)
		return initializedStores{}, err
	}
	defer func() { closeKVOnFailure(kvStore, &err) }()

	securedStore, secureErr := secureKV(kvStore, settings.Secrets)
	if secureErr != nil {
		slog.Error("Failed to enable at-rest secret encryption", "error", secureErr)
		return initializedStores{}, secureErr
	}
	kvStore = securedStore
	binaryStore, err := initBinaryStore(settings.Storage, catalog)
	if err != nil {
		slog.Error("Failed to create binary store", "error", err)
		return initializedStores{}, err
	}
	return initializedStores{catalog: catalog, kv: kvStore, binary: binaryStore}, nil
}

// secureKV enables at-rest encryption when an age identity is configured.
func secureKV(store kv.Store, settings SecretSettings) (kv.Store, error) {
	identity, err := secretbox.LoadAgeIdentity(
		os.Getenv("AYATO_SECRETS_AGE_IDENTITY"),
		settings.AgeIdentityFile,
	)
	if err != nil {
		return nil, errors.WrapErr(err, "failed to load the secrets age identity")
	}
	if identity == "" {
		return store, nil
	}
	box, err := secretbox.NewAgeX25519(identity)
	if err != nil {
		return nil, errors.WrapErr(err, "failed to build the secrets encryptor")
	}
	namespaces := settings.Namespaces
	if len(namespaces) == 0 {
		namespaces = defaultEncryptedNamespaces
	}
	slog.Info("at-rest secret encryption enabled", "namespaces", namespaces)
	return securekv.New(store, box, namespaces), nil
}

func initBinaryStore(
	settings StorageSettings,
	catalog *domain.RepositoryCatalog,
) (blob.Store, error) {
	repoNames := catalog.PhysicalNames()
	if settings.Backend == "s3" {
		slog.Warn("Using S3 is still experimental, please use with caution")
		return s3.New(&s3.Config{
			Bucket:          settings.S3.Bucket,
			Region:          settings.S3.Region,
			Endpoint:        settings.S3.Endpoint,
			AccessKeyID:     settings.S3.AccessKeyID,
			SecretAccessKey: settings.S3.SecretAccessKey,
			SessionToken:    settings.S3.SessionToken,
			UsePathStyle:    settings.S3.UsePathStyle,
			RepoNames:       repoNames,
		})
	}

	slog.Info("Using local file system as the binary store")
	return localfs.New(settings.LocalDir, repoNames), nil
}

func initKVStore(settings KVSettings) (kv.Store, error) {
	switch settings.Backend {
	case "sql", "external":
		slog.Warn("Using SQL is still experimental, please use with caution")
		return sqlkv.New(settings.SQLDriver, settings.SQLDSN)
	case "cfkv":
		slog.Warn("Using Cloudflare KV is still experimental, please use with caution")
		return cfkv.New(
			settings.Cloudflare.AccountID,
			settings.Cloudflare.Token,
			settings.Cloudflare.Namespace,
		)
	default:
		slog.Info("Using local BadgerDB as the default key-value store")
		return badgerkv.New(settings.BadgerPath)
	}
}
