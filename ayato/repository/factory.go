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
	"github.com/Hayao0819/Kamisato/ayato/repository/kv/securekv"
	"github.com/Hayao0819/Kamisato/ayato/repository/kv/sqlkv"
	"github.com/Hayao0819/Kamisato/internal/auth/secretbox"
	"github.com/Hayao0819/Kamisato/internal/conf"
	"github.com/Hayao0819/Kamisato/internal/errors"
	"github.com/Hayao0819/Kamisato/pkg/pacman/sign"
)

// defaultEncryptedNamespaces is the kv namespace set at-rest encryption seals when
// enabled; only the durable admin allowlist qualifies (sessions, tokens, replay and
// rate-limit state are stateless-signed or ephemeral).
var defaultEncryptedNamespaces = []string{allowNS}

// secureKV wraps store with at-rest encryption when an age identity is configured,
// else returns it unchanged. The key is read from the environment
// (AYATO_SECRETS_AGE_IDENTITY) or configured file, never the config value itself.
func secureKV(store kv.Store, cfg *conf.AyatoConfig) (kv.Store, error) {
	identity, err := secretbox.LoadAgeIdentity(os.Getenv("AYATO_SECRETS_AGE_IDENTITY"), cfg.Secrets.AgeIdentityFile)
	if err != nil {
		return nil, errors.WrapErr(err, "failed to load the secrets age identity")
	}
	if identity == "" {
		return store, nil
	}
	box, err := secretbox.NewAgeX25519(identity)
	if err != nil {
		return nil, errors.WrapErr(err, "failed to build the secrets encryptor")
	}
	ns := cfg.Secrets.Namespaces
	if len(ns) == 0 {
		ns = defaultEncryptedNamespaces
	}
	slog.Info("at-rest secret encryption enabled", "namespaces", ns)
	return securekv.New(store, box, ns), nil
}

// initBinaryStore keeps the IO layer conf-free by unpacking conf into the plain values the backends take.
func initBinaryStore(cfg *conf.AyatoConfig) (blob.Store, error) {
	// A tiered repo serves three pacman repos (its tiers), so the store must accept
	// every physical repo name, not just the logical one.
	repoNames := cfg.PhysicalRepoNames()

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
// second store against the same locked BadgerDB dir; the caller closes it.
func New(cfg *conf.AyatoConfig) (NameStore, BinaryRepository, AuthRepository, kv.Store, error) {
	kvStore, err := initKVStore(cfg)
	if err != nil {
		slog.Error("Failed to create key-value store", "error", err)
		return nil, nil, nil, nil, err
	}
	if cfg != nil {
		kvStore, err = secureKV(kvStore, cfg)
		if err != nil {
			slog.Error("Failed to enable at-rest secret encryption", "error", err)
			return nil, nil, nil, nil, err
		}
	}

	binStore, err := initBinaryStore(cfg)
	if err != nil {
		slog.Error("Failed to create binary store", "error", err)
		return nil, nil, nil, nil, err
	}

	var binOpts []BinaryRepoOption
	if cfg != nil && cfg.Sign.DB {
		signer, serr := loadDBSigner()
		if serr != nil {
			return nil, nil, nil, nil, serr
		}
		binOpts = append(binOpts, WithSigningTool(signer))
	}
	if cfg != nil {
		binOpts = append(binOpts, WithUpstreamRepos(cfg.UpstreamRepoNames()))
	}

	// Package files are stored directly under (repo, arch, filename); serializing
	// keeps per-(repo, arch) writes serialized.
	binRepo := NewBinaryRepository(newSerializingStore(binStore), binOpts...)
	return NewPackageMetadataRepo(kvStore), binRepo, NewAuthRepository(kvStore), kvStore, nil
}

// NewMigrationStores returns the raw kv and blob stores a migration job mutates,
// without the serving-path decorators (serializing, repository logic): migrations
// use kv.BulkStore and blob.ObjectMover directly, which the raw backends expose. The
// caller closes the kv store.
func NewMigrationStores(cfg *conf.AyatoConfig) (kv.Store, blob.Store, error) {
	kvStore, err := initKVStore(cfg)
	if err != nil {
		return nil, nil, err
	}
	if cfg != nil {
		if kvStore, err = secureKV(kvStore, cfg); err != nil {
			return nil, nil, err
		}
	}
	binStore, err := initBinaryStore(cfg)
	if err != nil {
		return nil, nil, err
	}
	return kvStore, binStore, nil
}

// loadDBSigner loads the repo-db signing key from the environment (never the config
// file, since it is a private key). Fails closed: sign.db enabled with no key is a
// startup error, so ayato never silently serves an unsigned database.
func loadDBSigner() (*openpgp.Entity, error) {
	armored := os.Getenv("AYATO_DB_SIGNING_KEY")
	if armored == "" {
		return nil, errors.NewErr("sign.db is enabled but AYATO_DB_SIGNING_KEY is unset")
	}
	entity, err := sign.LoadArmoredEntity(armored, os.Getenv("AYATO_DB_SIGNING_PASSPHRASE"))
	if err != nil {
		return nil, errors.WrapErr(err, "failed to load the repo-db signing key")
	}
	return entity, nil
}
