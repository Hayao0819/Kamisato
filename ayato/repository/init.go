package repository

import (
	"log/slog"

	"github.com/Hayao0819/Kamisato/ayato/domain"
	"github.com/Hayao0819/Kamisato/ayato/repository/binarystore/localfs"
	"github.com/Hayao0819/Kamisato/ayato/repository/binarystore/s3"
	"github.com/Hayao0819/Kamisato/ayato/repository/metastore/cloudflarekv"
	"github.com/Hayao0819/Kamisato/ayato/repository/metastore/localkv"
	"github.com/Hayao0819/Kamisato/ayato/repository/metastore/sql"
	"github.com/Hayao0819/Kamisato/internal/conf"
)

// initPkgBinaryStore initializes the binary store.
func initPkgBinaryStore(cfg *conf.AyatoConfig) (PkgBinaryStoreProvider, error) {
	var bin PkgBinaryStoreProvider
	var err error

	// S3が有効な場合はS3を利用
	if cfg.Store.StorageType == "s3" {
		slog.Warn("Using S3 is still experimental, please use with caution")
		bin, err = s3.NewS3(&cfg.Store.AWSS3)
		if err != nil {
			bin = nil
			slog.Error("Failed to create S3 client", "error", err)
		}
	}

	// S3が無効な場合はlocalfsを利用
	if bin == nil {
		slog.Info("Using local file system as the binary store")
		bin = localfs.NewLocalPkgBinaryStore(cfg)
	}

	return bin, nil
}

// initMetaStore initializes the meta store.
func initMetaStore(cfg *conf.AyatoConfig) (PkgNameStoreProvider, error) {
	var db PkgNameStoreProvider
	var err error

	switch cfg.Store.DBType {
	case "sql", "external":
		slog.Warn("Using SQL is still experimental, please use with caution")

		dsn, dsnerr := cfg.Store.SQL.DSN()
		if dsnerr != nil {
			slog.Debug("Failed to get DSN", "error", err)
		}

		db, err = sql.NewSql(cfg.Store.SQL.Driver, dsn)
	case "cfkv":
		slog.Warn("Using Cloudflare KV is still experimental, please use with caution")

		db, err = cloudflarekv.NewCloudflareKV(cfg.Store.CloudflareKV.AccountId, cfg.Store.CloudflareKV.Token, cfg.Store.CloudflareKV.Namespace)
	default:
		slog.Info("Using local BadgerDB as the default meta store")

		db, err = localkv.NewBadger(cfg.DbPath())
	}
	if err != nil {
		return nil, err
	}

	return db, nil
}

// New creates implementations of IPackageNameRepository and IPackageBinaryRepository.
func New(cfg *conf.AyatoConfig) (domain.IPackageNameRepository, domain.IPackageBinaryRepository, error) {
	// メタストア初期化
	db, err := initMetaStore(cfg)
	if err != nil {
		slog.Error("Failed to create meta store", "error", err)
		return nil, nil, err
	}

	// バイナリストア初期化
	bin, err := initPkgBinaryStore(cfg)
	if err != nil {
		slog.Error("Failed to create binary store", "error", err)
		return nil, nil, err
	}

	pkgNameRepo := &PackageNameRepository{
		pkgNameStore: db,
	}

	pkgBinaryRepo := &PackageBinaryRepository{
		pkgBinStore: bin,
		cfg:         cfg,
	}

	return pkgNameRepo, pkgBinaryRepo, nil
}
