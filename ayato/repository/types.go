package repository

import (
	"log/slog"

	"github.com/Hayao0819/Kamisato/ayato/repository/binarystore/localfs"
	"github.com/Hayao0819/Kamisato/ayato/repository/metastore/kv"
	"github.com/Hayao0819/Kamisato/ayato/repository/metastore/sql"
	"github.com/Hayao0819/Kamisato/ayato/repository/provider"
	"github.com/Hayao0819/Kamisato/conf"
)

type Repository struct {
	pkgNameStore provider.PkgNameStoreProvider
	pkgBinStore  provider.PkgBinaryStoreProvider
	cfg          *conf.AyatoConfig
}

var useS3 = false

func New(cfg *conf.AyatoConfig) (*Repository, error) {
	var db provider.PkgNameStoreProvider
	var err error

	dsn, err := cfg.Database.DSN()
	if err != nil {
		slog.Debug("Failed to get DSN", "error", err)
	}

	if dsn != "" {
		slog.Warn("Using SQL is still experimental, please use with caution")
		db, err = sql.NewSql(cfg.Database.Driver, dsn)
	} else {
		db, err = kv.NewBadger(cfg.DbPath())
	}
	if err != nil {
		return nil, err
	}

	// bin := localfs.NewLocalPkgBinaryStore(cfg)
	var bin provider.PkgBinaryStoreProvider
	if useS3 {
		// bin, err = s3.NewS3PkgBinaryStore(cfg)
	} else {
		bin = localfs.NewLocalPkgBinaryStore(cfg)
	}

	return &Repository{
		pkgNameStore: db,
		cfg:          cfg,
		pkgBinStore:  bin,
	}, nil
}
