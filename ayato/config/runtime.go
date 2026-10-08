package config

import (
	"fmt"

	"github.com/Hayao0819/Kamisato/ayato/repository"
	"github.com/Hayao0819/Kamisato/ayato/service"
)

func RepositorySettings(cfg *AyatoConfig) (repository.Settings, error) {
	if cfg == nil {
		return repository.Settings{}, fmt.Errorf("ayato config is nil")
	}
	catalog, err := cfg.RepositoryCatalog()
	if err != nil {
		return repository.Settings{}, fmt.Errorf("invalid repository catalog: %w", err)
	}
	settings := repository.Settings{
		Catalog:      catalog,
		SignDatabase: cfg.Sign.DB,
		Storage: repository.StorageSettings{
			Backend:  cfg.Store.StorageType,
			LocalDir: cfg.Store.LocalRepoDir,
			S3: repository.S3Settings{
				Bucket:          cfg.Store.AWSS3.Bucket,
				Region:          cfg.Store.AWSS3.Region,
				Endpoint:        cfg.Store.AWSS3.Endpoint,
				AccessKeyID:     cfg.Store.AWSS3.AccessKeyID,
				SecretAccessKey: cfg.Store.AWSS3.SecretAccessKey,
				SessionToken:    cfg.Store.AWSS3.SessionToken,
				UsePathStyle:    cfg.Store.AWSS3.UsePathStyle,
			},
		},
		KV: repository.KVSettings{
			Backend:    cfg.Store.DBType,
			BadgerPath: cfg.DbPath(),
			Cloudflare: repository.CloudflareKVSettings{
				AccountID: cfg.Store.CloudflareKV.AccountId,
				Token:     cfg.Store.CloudflareKV.Token,
				Namespace: cfg.Store.CloudflareKV.Namespace,
			},
		},
		Secrets: repository.SecretSettings{
			AgeIdentityFile: cfg.Secrets.AgeIdentityFile,
			Namespaces:      cfg.Secrets.Namespaces,
		},
	}
	if cfg.Store.DBType == "sql" || cfg.Store.DBType == "external" {
		settings.KV.SQLDriver = cfg.Store.SQL.Driver
		settings.KV.SQLDSN, err = cfg.Store.SQL.DSN()
		if err != nil {
			return repository.Settings{}, fmt.Errorf("configure SQL store: %w", err)
		}
	}
	return settings, nil
}

func ServiceSettings(cfg *AyatoConfig) service.Settings {
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
		TrustedVerificationKeys:    cfg.Verify.TrustedKeys,
		MasterVerificationKeys:     cfg.Verify.MasterKeys,
	}
}
