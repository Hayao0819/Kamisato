package repository

import (
	"log/slog"
	"os"

	"github.com/Hayao0819/Kamisato/ayato/domain"
	"github.com/Hayao0819/Kamisato/ayato/repository/blob"
	"github.com/Hayao0819/Kamisato/ayato/repository/blob/localfs"
	"github.com/Hayao0819/Kamisato/ayato/repository/blob/s3"
	"github.com/Hayao0819/Kamisato/ayato/repository/kv"
	"github.com/Hayao0819/Kamisato/ayato/repository/kv/badgerkv"
	"github.com/Hayao0819/Kamisato/ayato/repository/kv/cfkv"
	"github.com/Hayao0819/Kamisato/ayato/repository/kv/schema"
	"github.com/Hayao0819/Kamisato/ayato/repository/kv/securekv"
	"github.com/Hayao0819/Kamisato/ayato/repository/kv/sqlkv"
	"github.com/Hayao0819/Kamisato/internal/auth/secretbox"
	"github.com/Hayao0819/Kamisato/internal/conf"
	"github.com/Hayao0819/Kamisato/internal/errors"
)

var defaultEncryptedNamespaces = []string{schema.AdminAllowlist}

type initializedStores struct {
	catalog *domain.RepositoryCatalog
	kv      kv.Store
	binary  blob.Store
}

// initializeStores is the shared composition path for the runtime and migration
// commands. It owns cleanup until every component has initialized successfully.
func initializeStores(cfg *conf.AyatoConfig) (stores initializedStores, err error) {
	if cfg == nil {
		return initializedStores{}, errors.NewErr("ayato config is nil")
	}
	catalog, err := cfg.RepositoryCatalog()
	if err != nil {
		return initializedStores{}, errors.WrapErr(err, "invalid repository catalog")
	}

	kvStore, err := initKVStore(cfg)
	if err != nil {
		slog.Error("Failed to create key-value store", "error", err)
		return initializedStores{}, err
	}
	defer func() { closeKVOnFailure(kvStore, &err) }()

	securedStore, secureErr := secureKV(kvStore, cfg)
	if secureErr != nil {
		slog.Error("Failed to enable at-rest secret encryption", "error", secureErr)
		return initializedStores{}, secureErr
	}
	kvStore = securedStore
	binaryStore, err := initBinaryStore(cfg, catalog)
	if err != nil {
		slog.Error("Failed to create binary store", "error", err)
		return initializedStores{}, err
	}
	return initializedStores{catalog: catalog, kv: kvStore, binary: binaryStore}, nil
}

// secureKV enables at-rest encryption when an age identity is configured.
func secureKV(store kv.Store, cfg *conf.AyatoConfig) (kv.Store, error) {
	identity, err := secretbox.LoadAgeIdentity(
		os.Getenv("AYATO_SECRETS_AGE_IDENTITY"),
		cfg.Secrets.AgeIdentityFile,
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
	namespaces := cfg.Secrets.Namespaces
	if len(namespaces) == 0 {
		namespaces = defaultEncryptedNamespaces
	}
	slog.Info("at-rest secret encryption enabled", "namespaces", namespaces)
	return securekv.New(store, box, namespaces), nil
}

func initBinaryStore(
	cfg *conf.AyatoConfig,
	catalog *domain.RepositoryCatalog,
) (blob.Store, error) {
	repoNames := catalog.PhysicalNames()
	if cfg.Store.StorageType == "s3" {
		slog.Warn("Using S3 is still experimental, please use with caution")
		aws := cfg.Store.AWSS3
		return s3.New(&s3.Config{
			Bucket:          aws.Bucket,
			Region:          aws.Region,
			Endpoint:        aws.Endpoint,
			AccessKeyID:     aws.AccessKeyID,
			SecretAccessKey: aws.SecretAccessKey,
			SessionToken:    aws.SessionToken,
			UsePathStyle:    aws.UsePathStyle,
			RepoNames:       repoNames,
		})
	}

	slog.Info("Using local file system as the binary store")
	return localfs.New(cfg.Store.LocalRepoDir, repoNames), nil
}

func initKVStore(cfg *conf.AyatoConfig) (kv.Store, error) {
	switch cfg.Store.DBType {
	case "sql", "external":
		slog.Warn("Using SQL is still experimental, please use with caution")
		dsn, err := cfg.Store.SQL.DSN()
		if err != nil {
			slog.Debug("Failed to get DSN", "error", err)
		}
		return sqlkv.New(cfg.Store.SQL.Driver, dsn)
	case "cfkv":
		slog.Warn("Using Cloudflare KV is still experimental, please use with caution")
		config := cfg.Store.CloudflareKV
		return cfkv.New(config.AccountId, config.Token, config.Namespace)
	default:
		slog.Info("Using local BadgerDB as the default key-value store")
		return badgerkv.New(cfg.DbPath())
	}
}
