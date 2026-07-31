package service_test

import (
	ayatoconfig "github.com/Hayao0819/Kamisato/ayato/config"
	"github.com/Hayao0819/Kamisato/ayato/service"
)

func settingsFromConfig(cfg *ayatoconfig.AyatoConfig) service.Settings {
	if cfg == nil {
		return service.Settings{}
	}
	catalog, catalogErr := cfg.RepositoryCatalog()
	return service.Settings{
		Catalog:                    catalog,
		CatalogError:               catalogErr,
		RequireSign:                cfg.RequireSign,
		RequireBuildinfoProvenance: cfg.RequireBuildinfoProvenance,
		ExpectedBuildDir:           cfg.ExpectedBuildDir(),
		ProtectedNames:             cfg.ProtectedNames,
		MaxBatchPackages:           cfg.MaxBatchPackages,
		MaxPackageSize:             cfg.MaxSize,
		SignDatabase:               cfg.Sign.DB,
		VerificationKeyring:        cfg.Verify.Keyring,
		TrustedVerificationKeys:    cfg.Verify.TrustedKeys,
		MasterVerificationKeys:     cfg.Verify.MasterKeys,
	}
}
