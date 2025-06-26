package repository

import (
	"log/slog"

	"github.com/Hayao0819/Kamisato/ayato/repository/binarystore/localfs"
	"github.com/Hayao0819/Kamisato/ayato/repository/binarystore/s3"
	"github.com/Hayao0819/Kamisato/ayato/repository/metastore/cloudflarekv"
	"github.com/Hayao0819/Kamisato/ayato/repository/metastore/localkv"
	"github.com/Hayao0819/Kamisato/ayato/repository/metastore/sql"
	"github.com/Hayao0819/Kamisato/internal/conf"
)

func initPkgBinaryStore(cfg *conf.AyatoConfig) (PkgBinaryStoreProvider, error) {
	var bin PkgBinaryStoreProvider
	var err error

	// Check if S3 is enabled
	if cfg.Store.StorageType == "s3" {
		slog.Warn("Using S3 is still experimental, please use with caution")
		bin, err = s3.NewS3(&cfg.Store.AWSS3)
		if err != nil {
			bin = nil
			slog.Error("Failed to create S3 client", "error", err)
		}
	}

	// Fallback to localfs if S3 is not enabled
	if bin == nil {
		slog.Info("Using local file system as the binary store")
		bin = localfs.NewLocalPkgBinaryStore(cfg)
	}

	return bin, nil
}

func initMetaStore(cfg *conf.AyatoConfig) (PkgNameStoreProvider, error) {
	var db PkgNameStoreProvider
	var err error

	if cfg.Store.DBType == "sql" || cfg.Store.DBType == "external" {

		slog.Warn("Using SQL is still experimental, please use with caution")

		dsn, dsnerr := cfg.Store.SQL.DSN()
		if dsnerr != nil {
			slog.Debug("Failed to get DSN", "error", err)
		}

		db, err = sql.NewSql(cfg.Store.SQL.Driver, dsn)
	} else if cfg.Store.DBType == "cfkv" {

		slog.Warn("Using Cloudflare KV is still experimental, please use with caution")

		db, err = cloudflarekv.NewCloudflareKV(cfg.Store.CloudflareKV.AccountId, cfg.Store.CloudflareKV.Token, cfg.Store.CloudflareKV.Namespace)
	} else {

		slog.Info("Using local BadgerDB as the default meta store")

		db, err = localkv.NewBadger(cfg.DbPath())
	}
	if err != nil {
		return nil, err
	}

	return db, nil
}

func New(cfg *conf.AyatoConfig) (IRepository, error) {
	// Initialize the database
	db, err := initMetaStore(cfg)
	if err != nil {
		slog.Error("Failed to create meta store", "error", err)
		return nil, err
	}

	// Check if S3 is enabled
	bin, err := initPkgBinaryStore(cfg)
	if err != nil {
		slog.Error("Failed to create binary store", "error", err)
		return nil, err
	}

	return &Repository{
		pkgNameStore: db,
		cfg:          cfg,
		pkgBinStore:  bin,
	}, nil
}
