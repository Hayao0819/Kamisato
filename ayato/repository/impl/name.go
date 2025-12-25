package impl

import (
	"github.com/Hayao0819/Kamisato/ayato/repository/provider"
)

// PackageNameRepository is the implementation struct for package name management operations.
type PackageNameRepository struct {
	pkgNameStore provider.PkgNameStoreProvider // Store for package names and file names
}

// NewPackageNameRepository creates a new PackageNameRepository instance.
func NewPackageNameRepository(pkgNameStore provider.PkgNameStoreProvider) *PackageNameRepository {
	return &PackageNameRepository{
		pkgNameStore: pkgNameStore,
	}
}
