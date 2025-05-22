package repository

import (
	"github.com/Hayao0819/Kamisato/conf"
)

type Repository struct {
	pkgNameStore PkgNameStoreProvider
	pkgBinStore  PkgBinaryStoreProvider
	cfg          *conf.AyatoConfig
}

var useS3 = false
