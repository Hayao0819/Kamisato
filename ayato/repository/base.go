package repository

import (
	"github.com/Hayao0819/Kamisato/internal/conf"
)

// PackageNameRepository is the implementation struct for package name management operations.
type PackageNameRepository struct {
	pkgNameStore PkgNameStoreProvider // Store for package names and file names
}

// PackageBinaryRepository is the implementation struct for package binary operations.
type PackageBinaryRepository struct {
	pkgBinStore PkgBinaryStoreProvider // Binary file store
	cfg         *conf.AyatoConfig      // Configuration
}
