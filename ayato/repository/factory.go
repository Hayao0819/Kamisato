package repository

import (
	"log/slog"

	"github.com/Hayao0819/Kamisato/ayato/repository/namestore/cloudflarekv"
	"github.com/Hayao0819/Kamisato/ayato/repository/namestore/localkv"
	"github.com/Hayao0819/Kamisato/ayato/repository/namestore/sql"
	"github.com/Hayao0819/Kamisato/ayato/repository/store/localfs"
	"github.com/Hayao0819/Kamisato/ayato/repository/store/s3"
	"github.com/Hayao0819/Kamisato/internal/conf"
)

// initBinaryStore initializes the low-level binary store backend.
func initBinaryStore(cfg *conf.AyatoConfig) (Store, error) {
	if cfg.Store.StorageType == "s3" {
		slog.Warn("Using S3 is still experimental, please use with caution")
		if bin, err := s3.New(&cfg.Store.AWSS3); err != nil {
			slog.Error("Failed to create S3 client, falling back to local file system", "error", err)
		} else {
			return bin, nil
		}
	}

	slog.Info("Using local file system as the binary store")
	return localfs.New(cfg), nil
}

// initNameStore initializes the package-name (metadata) store backend.
func initNameStore(cfg *conf.AyatoConfig) (NameStore, error) {
	switch cfg.Store.DBType {
	case "sql", "external":
		slog.Warn("Using SQL is still experimental, please use with caution")
		dsn, err := cfg.Store.SQL.DSN()
		if err != nil {
			slog.Debug("Failed to get DSN", "error", err)
		}
		return sql.NewSql(cfg.Store.SQL.Driver, dsn)
	case "cfkv":
		slog.Warn("Using Cloudflare KV is still experimental, please use with caution")
		return cloudflarekv.NewCloudflareKV(cfg.Store.CloudflareKV.AccountId, cfg.Store.CloudflareKV.Token, cfg.Store.CloudflareKV.Namespace)
	default:
		slog.Info("Using local BadgerDB as the default meta store")
		return localkv.NewBadger(cfg.DbPath())
	}
}

// New creates the NameStore and BinaryRepository used by the service.
func New(cfg *conf.AyatoConfig) (NameStore, BinaryRepository, error) {
	nameStore, err := initNameStore(cfg)
	if err != nil {
		slog.Error("Failed to create meta store", "error", err)
		return nil, nil, err
	}

	binStore, err := initBinaryStore(cfg)
	if err != nil {
		slog.Error("Failed to create binary store", "error", err)
		return nil, nil, err
	}

	return nameStore, NewBinaryRepository(binStore, cfg), nil
}
