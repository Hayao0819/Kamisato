package repository

import (
	"log/slog"

	"github.com/Hayao0819/Kamisato/ayato/repository/blob"
	"github.com/Hayao0819/Kamisato/ayato/repository/blob/localfs"
	"github.com/Hayao0819/Kamisato/ayato/repository/blob/s3"
	"github.com/Hayao0819/Kamisato/ayato/repository/kv"
	"github.com/Hayao0819/Kamisato/ayato/repository/kv/badgerkv"
	"github.com/Hayao0819/Kamisato/ayato/repository/kv/cfkv"
	"github.com/Hayao0819/Kamisato/ayato/repository/kv/sqlkv"
	"github.com/Hayao0819/Kamisato/internal/conf"
)

// initBinaryStore initializes the low-level binary store backend. It unpacks the
// conf into the plain values the backends take, keeping the IO layer conf-free
// (mirroring the kv backends).
func initBinaryStore(cfg *conf.AyatoConfig) (blob.Store, error) {
	if cfg.Store.StorageType == "s3" {
		slog.Warn("Using S3 is still experimental, please use with caution")
		a := cfg.Store.AWSS3
		if bin, err := s3.New(&s3.Config{
			Bucket:          a.Bucket,
			Region:          a.Region,
			Endpoint:        a.Endpoint,
			AccessKeyID:     a.AccessKeyID,
			SecretAccessKey: a.SecretAccessKey,
			SessionToken:    a.SessionToken,
			UsePathStyle:    a.UsePathStyle,
		}); err != nil {
			slog.Error("Failed to create S3 client, falling back to local file system", "error", err)
		} else {
			return bin, nil
		}
	}

	slog.Info("Using local file system as the binary store")
	repoNames := make([]string, 0, len(cfg.Repos))
	for _, r := range cfg.Repos {
		repoNames = append(repoNames, r.Name)
	}
	return localfs.New(cfg.Store.LocalRepoDir, repoNames), nil
}

// initKVStore initializes the shared generic key-value store. The same store
// backs package metadata and (later) other app data; backend is selected by
// cfg.Store.DBType.
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
		return cfkv.New(cfg.Store.CloudflareKV.AccountId, cfg.Store.CloudflareKV.Token, cfg.Store.CloudflareKV.Namespace)
	default:
		slog.Info("Using local BadgerDB as the default key-value store")
		return badgerkv.New(cfg.DbPath())
	}
}

// New creates the NameStore, BinaryRepository, and AuthRepository used by the
// service, all riding a single shared kv.Store built here. That store is also
// returned so additional consumers (the optional AUR backend) can partition
// their own data under separate namespaces rather than opening a second store
// against the same locked BadgerDB directory; the caller closes it on exit.
func New(cfg *conf.AyatoConfig) (NameStore, BinaryRepository, AuthRepository, kv.Store, error) {
	kvStore, err := initKVStore(cfg)
	if err != nil {
		slog.Error("Failed to create key-value store", "error", err)
		return nil, nil, nil, nil, err
	}

	binStore, err := initBinaryStore(cfg)
	if err != nil {
		slog.Error("Failed to create binary store", "error", err)
		return nil, nil, nil, nil, err
	}

	return NewPackageMetadataRepo(kvStore), NewBinaryRepository(newSerializingStore(binStore)), NewAuthRepository(kvStore), kvStore, nil
}
