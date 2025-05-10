package repository

import (
	"github.com/BrenekH/blinky"
	"github.com/Hayao0819/Kamisato/ayato/repository/kv"
	"github.com/Hayao0819/Kamisato/conf"
)

// type Repository blinky.PackageNameToFileProvider

type Repository struct {
	pkgNameDb blinky.PackageNameToFileProvider
	cfg       *conf.AyatoConfig
}

func New(cfg *conf.AyatoConfig) (*Repository, error) {
	db, err := kv.NewBadger(cfg.DbPath())
	if err != nil {
		return nil, err
	}

	return &Repository{
		pkgNameDb: db,
		cfg:       cfg,
	}, nil
}
