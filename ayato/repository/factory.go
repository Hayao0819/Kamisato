package repository

import (
	"os"

	"github.com/ProtonMail/go-crypto/openpgp"

	"github.com/Hayao0819/Kamisato/ayato/repository/blob"
	"github.com/Hayao0819/Kamisato/ayato/repository/kv"
	"github.com/Hayao0819/Kamisato/internal/conf"
	"github.com/Hayao0819/Kamisato/internal/errors"
	"github.com/Hayao0819/Kamisato/pkg/pacman/sign"
)

func closeKVOnFailure(store kv.Store, err *error) {
	if store == nil || err == nil || *err == nil {
		return
	}
	if closeErr := store.Close(); closeErr != nil {
		*err = errors.Join(*err, errors.WrapErr(closeErr, "failed to close key-value store after initialization failure"))
	}
}

// New returns the shared kv.Store alongside the repositories so other consumers
// (e.g. the AUR backend) can partition their own namespaces instead of opening a
// second store against the same locked BadgerDB dir; the caller closes it.
func New(cfg *conf.AyatoConfig) (
	nameStore NameStore,
	binaryRepo BinaryRepository,
	authRepo AuthRepository,
	returnedStore kv.Store,
	err error,
) {
	stores, err := initializeStores(cfg)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	defer func() { closeKVOnFailure(stores.kv, &err) }()

	var binOpts []BinaryRepoOption
	if cfg != nil && cfg.Sign.DB {
		signer, serr := loadDBSigner()
		if serr != nil {
			return nil, nil, nil, nil, serr
		}
		binOpts = append(binOpts, WithSigningTool(signer))
	}
	binOpts = append(binOpts, WithUpstreamRepos(stores.catalog.UpstreamPhysicalNames()))
	binOpts = append(binOpts, WithStagedUploads(stores.binary))

	// Package files are stored directly under (repo, arch, filename); serializing
	// keeps per-(repo, arch) writes serialized.
	binRepo := NewBinaryRepository(newSerializingStore(stores.binary), binOpts...)
	return NewPackageMetadataRepo(stores.kv), binRepo, NewAuthRepository(stores.kv), stores.kv, nil
}

// NewMigrationStores returns migration-facing stores. The blob store is raw so
// ObjectMover remains available. The KV store retains securekv because migrations
// operate on logical plaintext values; securekv preserves BulkStore while sealing
// sensitive namespaces before the backend receives a batch. The caller closes it.
func NewMigrationStores(cfg *conf.AyatoConfig) (returnedKV kv.Store, returnedBlob blob.Store, err error) {
	stores, err := initializeStores(cfg)
	if err != nil {
		return nil, nil, err
	}
	return stores.kv, stores.binary, nil
}

// NewRawKV returns the kv store without the securekv decorator, for maintenance that
// operates on raw keys (kv.KeyAuditor) rather than values. The caller closes it.
func NewRawKV(cfg *conf.AyatoConfig) (kv.Store, error) {
	return initKVStore(cfg)
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
