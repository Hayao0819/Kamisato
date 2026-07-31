package app

import (
	"context"
	"time"

	ayatoconfig "github.com/Hayao0819/Kamisato/ayato/config"
	"github.com/Hayao0819/Kamisato/ayato/migrate"
	"github.com/Hayao0819/Kamisato/ayato/repository"
	"github.com/Hayao0819/Kamisato/ayato/repository/kv"
	"github.com/Hayao0819/Kamisato/ayato/service"
	"github.com/Hayao0819/Kamisato/internal/errors"
)

func Audit(cfg *ayatoconfig.AyatoConfig, prune bool) (foreign []string, returnedErr error) {
	repoSettings, err := RepositorySettings(cfg)
	if err != nil {
		return nil, err
	}
	store, err := repository.NewRawKV(repoSettings.KV)
	if err != nil {
		return nil, errors.WrapErr(err, "failed to open kv store")
	}
	defer func() { returnedErr = errors.Join(returnedErr, store.Close()) }()

	auditor, ok := store.(kv.KeyAuditor)
	if !ok {
		return nil, errors.New("the configured kv backend does not support key auditing")
	}
	foreign, err = auditor.ForeignKeys()
	if err != nil || !prune || len(foreign) == 0 {
		return foreign, err
	}
	return foreign, auditor.DeleteRawKeys(foreign)
}

func GC(cfg *ayatoconfig.AyatoConfig, repoName string, olderThan time.Duration, deleteObjects bool) (orphans []service.OrphanObject, returnedErr error) {
	if olderThan < 0 {
		return nil, errors.New("orphan age must not be negative")
	}
	repoSettings, err := RepositorySettings(cfg)
	if err != nil {
		return nil, err
	}
	pkgNameRepo, pkgBinaryRepo, authRepo, kvStore, err := repository.New(repoSettings)
	if err != nil {
		return nil, errors.WrapErr(err, "failed to initialize repository")
	}
	defer func() { returnedErr = errors.Join(returnedErr, kvStore.Close()) }()

	signerRepo := repository.NewSignerRepository(kvStore)
	s := service.New(pkgNameRepo, pkgBinaryRepo, authRepo, signerRepo, ServiceSettings(cfg))
	return s.ReconcileOrphans(repoName, olderThan, !deleteObjects)
}

type MigrationResult struct {
	Status *migrate.Status
	Run    migrate.Result
}

func Migrate(ctx context.Context, cfg *ayatoconfig.AyatoConfig, status bool, options migrate.RunOptions) (result MigrationResult, returnedErr error) {
	repoSettings, err := RepositorySettings(cfg)
	if err != nil {
		return result, err
	}
	kvStore, blobStore, err := repository.NewMigrationStores(repoSettings)
	if err != nil {
		return result, errors.WrapErr(err, "failed to open stores")
	}
	defer func() { returnedErr = errors.Join(returnedErr, kvStore.Close()) }()

	if status {
		migrationStatus, err := migrate.Statuses(kvStore, migrate.Registered())
		result.Status = &migrationStatus
		return result, err
	}
	result.Run, returnedErr = migrate.Run(ctx, &migrate.Stores{KV: kvStore, Blob: blobStore}, migrate.Registered(), options)
	return result, returnedErr
}
