package repository

import (
	"github.com/Hayao0819/Kamisato/internal/conf"
)

type Repository struct {
	pkgNameStore PkgNameStoreProvider
	pkgBinStore  PkgBinaryStoreProvider
	cfg          *conf.AyatoConfig
}
