package repository

import (
	"log/slog"

	"github.com/BrenekH/blinky"
	"github.com/Hayao0819/Kamisato/ayato/repository/metastore/kv"
	"github.com/Hayao0819/Kamisato/ayato/repository/binarystore/localfs"
	"github.com/Hayao0819/Kamisato/ayato/repository/metastore/sql"
	"github.com/Hayao0819/Kamisato/conf"
)

type Repository struct {
	pkgNameStore PkgNameStoreProvider
	pkgBinStore  PkgBinaryStoreProvider
	cfg          *conf.AyatoConfig
}

func New(cfg *conf.AyatoConfig) (*Repository, error) {
	var db PkgNameStoreProvider
	var err error
	if cfg.Database.Server != "" {
		slog.Warn("Using SQL is still experimental, please use with caution")
		db, err = sql.NewSql(cfg.Database.Driver, cfg.Database.Server)
	} else {
		db, err = kv.NewBadger(cfg.DbPath())
	}
	if err != nil {
		return nil, err
	}

	bin := localfs.NewLocalPkgBinaryStore(cfg)

	return &Repository{
		pkgNameStore: db,
		cfg:          cfg,
		pkgBinStore:  bin,
	}, nil
}

type PkgNameStoreProvider blinky.PackageNameToFileProvider

type PkgBinaryStoreProvider interface {
	// StoreFile stores a file
	StoreFile(repo string, arch string, file string, useSignedDB bool, gnupgDir *string) error

	// DeleteFile deletes a file from the database
	DeleteFile(repo string, arch string, file string, useSignedDB bool, gnupgDir *string) error

	// Init initializes the database.
	Init(name string, arch string, useSignedDB bool, gnupgDir *string) error
	RepoNames() ([]string, error)
	Files(repo string, arch string) ([]string, error)
	ExistArchs(repo string) ([]string, error)
}
