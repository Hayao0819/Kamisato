package repository

import (
	"github.com/Hayao0819/Kamisato/ayato/repository/kv"
	"github.com/Hayao0819/Kamisato/ayato/repository/kv/schema"
	"github.com/Hayao0819/Kamisato/internal/errors"
)

//go:generate mockgen -source=namestore.go -destination=../test/mocks/namestore.go -package=mocks

// NameStore maps a package's (repo, arch, name) to its stored file name; arch is
// "any" for an arch=any package, else the concrete arch. Keying by repo keeps the
// same package name distinct across the tiers of a tiered repo (staging/testing/
// stable are separate physical repos), and by arch keeps it distinct across arches
// (pacman identity is the (pkgname, arch) tuple).
type NameStore interface {
	PackageFile(repo, arch, name string) (string, error)
	StorePackageFile(repo, arch, packageName, filePath string) error
	// StorePackageFiles records many entries under one repo in a single backend
	// write when the store supports it, so a batch publish spends one request
	// instead of one per package. Entries never expire, like StorePackageFile.
	StorePackageFiles(repo string, entries []PackageFileEntry) error
	DeletePackageFileEntry(repo, arch, packageName string) error
}

// PackageFileEntry is one (arch, name) -> file-name mapping for a batched write.
type PackageFileEntry struct {
	Arch, Name, FileName string
}

type packageMetadataRepo struct {
	kv kv.Store
}

func NewPackageMetadataRepo(s kv.Store) NameStore {
	return &packageMetadataRepo{kv: s}
}

func nameKey(repo, arch, name string) string {
	return repo + "/" + arch + "/" + name
}

// PackageFile reports a miss as ("", nil), not an error, so callers keep their
// read-through-on-miss behaviour.
func (r *packageMetadataRepo) PackageFile(repo, arch, name string) (string, error) {
	v, err := r.kv.Get(schema.PackageFiles, nameKey(repo, arch, name))
	if errors.Is(err, kv.ErrNotFound) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return string(v), nil
}

// The entry never expires (ttl 0): it is durable metadata, not a cache line.
func (r *packageMetadataRepo) StorePackageFile(repo, arch, packageName, filePath string) error {
	return r.kv.Set(schema.PackageFiles, nameKey(repo, arch, packageName), []byte(filePath), 0)
}

// StorePackageFiles uses the backend's BulkStore path when available (cfkv sends
// them in one bulk request), else writes per key. A store that cannot batch is
// still correct, just without the request saving.
func (r *packageMetadataRepo) StorePackageFiles(repo string, items []PackageFileEntry) error {
	if len(items) == 0 {
		return nil
	}
	entries := make([]kv.Entry, len(items))
	for i, it := range items {
		entries[i] = kv.Entry{Key: nameKey(repo, it.Arch, it.Name), Value: []byte(it.FileName)}
	}
	if b, ok := r.kv.(kv.BulkStore); ok {
		return b.BulkSet(schema.PackageFiles, entries, 0)
	}
	for _, e := range entries {
		if err := r.kv.Set(schema.PackageFiles, e.Key, e.Value, 0); err != nil {
			return err
		}
	}
	return nil
}

func (r *packageMetadataRepo) DeletePackageFileEntry(repo, arch, packageName string) error {
	return r.kv.Delete(schema.PackageFiles, nameKey(repo, arch, packageName))
}
