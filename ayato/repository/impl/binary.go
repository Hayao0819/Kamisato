package impl

import (
	"github.com/Hayao0819/Kamisato/ayato/repository/provider"
	"github.com/Hayao0819/Kamisato/internal/conf"
)

// PackageBinaryRepository is the implementation struct for package binary operations.
type PackageBinaryRepository struct {
	pkgBinStore provider.PkgBinaryStoreProvider // Binary file store
	cfg         *conf.AyatoConfig                // Configuration
}

// NewPackageBinaryRepository creates a new PackageBinaryRepository instance.
func NewPackageBinaryRepository(pkgBinStore provider.PkgBinaryStoreProvider, cfg *conf.AyatoConfig) *PackageBinaryRepository {
	return &PackageBinaryRepository{
		pkgBinStore: pkgBinStore,
		cfg:         cfg,
	}
}
