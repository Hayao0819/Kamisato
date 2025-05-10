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

func New(dbDirPath string, cfg *conf.AyatoConfig) (*Repository, error) {
	db, err := badger.Open(badger.DefaultOptions(dbDirPath))
	if err != nil {
		return nil, err
	}

	// return &BadgerRepository{db: db}, nil
	return &Repository{
		pkgNameDb: &BadgerRepository{db: db},
		cfg:       nil,
	}, nil
}
