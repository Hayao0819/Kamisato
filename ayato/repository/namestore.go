package repository

import (
	"github.com/Hayao0819/Kamisato/ayato/repository/kv"
	"github.com/Hayao0819/Kamisato/ayato/service"
)

//go:generate mockgen -source=namestore.go -destination=../test/mocks/namestore.go -package=mocks

// NameStore maps a package's (repo, arch, name) to its stored file name; arch is
// "any" for an arch=any package, else the concrete arch. Keying by repo keeps the
// same package name distinct across the tiers of a tiered repo (staging/testing/
// stable are separate physical repos), and by arch keeps it distinct across arches
// (pacman identity is the (pkgname, arch) tuple).
type NameStore = service.NameStore
type PackageFileEntry = service.PackageFileEntry

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
	v, ok, err := getOptional(
		r.kv,
		kv.PackageFiles,
		nameKey(repo, arch, name),
		"package metadata: get file",
	)
	if err != nil {
		return "", err
	}
	if !ok {
		return "", nil
	}
	return string(v), nil
}

// The entry never expires (ttl 0): it is durable metadata, not a cache line.
func (r *packageMetadataRepo) StorePackageFile(repo, arch, packageName, filePath string) error {
	return r.kv.Set(kv.PackageFiles, nameKey(repo, arch, packageName), []byte(filePath), 0)
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
		return b.BulkSet(kv.PackageFiles, entries, 0)
	}
	for _, e := range entries {
		if err := r.kv.Set(kv.PackageFiles, e.Key, e.Value, 0); err != nil {
			return err
		}
	}
	return nil
}

func (r *packageMetadataRepo) DeletePackageFileEntry(repo, arch, packageName string) error {
	return r.kv.Delete(kv.PackageFiles, nameKey(repo, arch, packageName))
}
