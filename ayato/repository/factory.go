package repository

import (
	"log/slog"
	"os"

	"github.com/ProtonMail/go-crypto/openpgp"

	"github.com/Hayao0819/Kamisato/ayato/repository/blob"
	"github.com/Hayao0819/Kamisato/ayato/repository/blob/localfs"
	"github.com/Hayao0819/Kamisato/ayato/repository/blob/s3"
	"github.com/Hayao0819/Kamisato/ayato/repository/kv"
	"github.com/Hayao0819/Kamisato/ayato/repository/kv/badgerkv"
	"github.com/Hayao0819/Kamisato/ayato/repository/kv/cfkv"
	"github.com/Hayao0819/Kamisato/ayato/repository/kv/sqlkv"
	"github.com/Hayao0819/Kamisato/internal/conf"
	"github.com/Hayao0819/Kamisato/internal/errwrap"
	"github.com/Hayao0819/Kamisato/pkg/pacman/sign"
)

// initBinaryStore keeps the IO layer conf-free by unpacking conf into the plain values the backends take.
func initBinaryStore(cfg *conf.AyatoConfig) (blob.Store, error) {
	repoNames := make([]string, 0, len(cfg.Repos)+1)
	for _, r := range cfg.Repos {
		repoNames = append(repoNames, r.Name)
	}
	// The pool's reserved repo must be an accepted key so package bytes can be
	// content-addressed under it; poolStore filters it from the servable repo set.
	repoNames = append(repoNames, poolRepo)

	if cfg.Store.StorageType == "s3" {
		slog.Warn("Using S3 is still experimental, please use with caution")
		a := cfg.Store.AWSS3
		bin, err := s3.New(&s3.Config{
			Bucket:          a.Bucket,
			Region:          a.Region,
			Endpoint:        a.Endpoint,
			AccessKeyID:     a.AccessKeyID,
			SecretAccessKey: a.SecretAccessKey,
			SessionToken:    a.SessionToken,
			UsePathStyle:    a.UsePathStyle,
			RepoNames:       repoNames,
		})
		if err != nil {
			// Fail closed: silently downgrading to localfs would put durable state on
			// ephemeral Cloud Run disk and lose data. localfs is only used when it is
			// the configured backend, never as an implicit S3 fallback.
			return nil, err
		}
		return bin, nil
	}

	slog.Info("Using local file system as the binary store")
	return localfs.New(cfg.Store.LocalRepoDir, repoNames), nil
}

func initKVStore(cfg *conf.AyatoConfig) (kv.Store, error) {
	switch cfg.Store.DBType {
	case "sql", "external":
		slog.Warn("Using SQL is still experimental, please use with caution")
		dsn, err := cfg.Store.SQL.DSN()
		if err != nil {
			slog.Debug("Failed to get DSN", "error", err)
		}
		return sqlkv.New(cfg.Store.SQL.Driver, dsn)
	case "cfkv":
		slog.Warn("Using Cloudflare KV is still experimental, please use with caution")
		return cfkv.New(cfg.Store.CloudflareKV.AccountId, cfg.Store.CloudflareKV.Token, cfg.Store.CloudflareKV.Namespace)
	default:
		slog.Info("Using local BadgerDB as the default key-value store")
		return badgerkv.New(cfg.DbPath())
	}
}

// New returns the shared kv.Store alongside the repositories so other consumers
// (e.g. the AUR backend) can partition their own namespaces instead of opening a
// second store against the same locked BadgerDB dir; the caller closes it. The
// returned PoolCollector runs the pool retention GC (nil when pooling is disabled).
func New(cfg *conf.AyatoConfig) (NameStore, BinaryRepository, AuthRepository, kv.Store, PoolCollector, error) {
	kvStore, err := initKVStore(cfg)
	if err != nil {
		slog.Error("Failed to create key-value store", "error", err)
		return nil, nil, nil, nil, nil, err
	}

	binStore, err := initBinaryStore(cfg)
	if err != nil {
		slog.Error("Failed to create binary store", "error", err)
		return nil, nil, nil, nil, nil, err
	}

	var binOpts []BinaryRepoOption
	if cfg != nil && cfg.Sign.DB {
		signer, serr := loadDBSigner()
		if serr != nil {
			return nil, nil, nil, nil, nil, serr
		}
		binOpts = append(binOpts, WithSigningTool(signer))
	}

	// The pool decorates the raw store so package writes are content-addressed and
	// reads resolve through pointers; serializing wraps it so per-(repo, arch)
	// writes stay serialized exactly as before.
	var pool *poolStore
	inner := binStore
	if cfg == nil || cfg.PoolEnabled() {
		pool = newPoolStore(binStore, kvStore)
		inner = pool
	}
	var collector PoolCollector
	if pool != nil {
		collector = pool
	}

	binRepo := NewBinaryRepository(newSerializingStore(inner), binOpts...)
	return NewPackageMetadataRepo(kvStore), binRepo, NewAuthRepository(kvStore), kvStore, collector, nil
}

// loadDBSigner loads the repo-db signing key from the environment (never the
// config file, since it is a private key), unlocking it with an optional
// passphrase. It fails closed: sign.db enabled with no key is a startup error, so
// ayato never silently serves an unsigned database when a signed one was asked for.
func loadDBSigner() (*openpgp.Entity, error) {
	armored := os.Getenv("AYATO_DB_SIGNING_KEY")
	if armored == "" {
		return nil, errwrap.NewErr("sign.db is enabled but AYATO_DB_SIGNING_KEY is unset")
	}
	entity, err := sign.LoadArmoredEntity(armored, os.Getenv("AYATO_DB_SIGNING_PASSPHRASE"))
	if err != nil {
		return nil, errwrap.WrapErr(err, "failed to load the repo-db signing key")
	}
	return entity, nil
}
