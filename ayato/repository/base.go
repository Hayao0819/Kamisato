package repository

import (
	"github.com/Hayao0819/Kamisato/internal/conf"
)

// Repository is the main implementation struct for repository operations.
type Repository struct {
	pkgNameStore PkgNameStoreProvider   // Store for package names and file names
	pkgBinStore  PkgBinaryStoreProvider // Binary file store
	cfg          *conf.AyatoConfig      // Configuration
}
