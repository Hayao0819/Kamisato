package repository

import (
	"github.com/BrenekH/blinky"
	"github.com/Hayao0819/Kamisato/conf"
	"github.com/dgraph-io/badger/v3"
)

// type Repository blinky.PackageNameToFileProvider

type Repository struct {
	pkgNameDb blinky.PackageNameToFileProvider
	cfg       *conf.AyatoConfig
}

func New(cfg *conf.AyatoConfig) (*Repository, error) {
	db, err := badger.Open(badger.DefaultOptions(cfg.DbPath()))
	if err != nil {
		return nil, err
	}

	return &Repository{
		pkgNameDb: &BadgerRepository{db: db},
		cfg:       cfg,
	}, nil
}
