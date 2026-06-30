package repository

import (
	"errors"

	"github.com/Hayao0819/Kamisato/ayato/repository/kv"
)

//go:generate mockgen -source=namestore.go -destination=../test/mocks/namestore.go -package=mocks

// NameStore maps a package's (arch, name) to its stored file name; arch is "any"
// for an arch=any package, else the concrete arch. Keying by (arch, name) keeps the
// same package name distinct across arches (pacman identity is the (pkgname, arch) tuple).
type NameStore interface {
	PackageFile(arch, name string) (string, error)
	StorePackageFile(arch, packageName, filePath string) error
	DeletePackageFileEntry(arch, packageName string) error
}

// pkgMetadataNamespace isolates package-name -> file-name entries from other consumers (e.g. auth) on the shared kv.Store.
const pkgMetadataNamespace = "pkgfile"

type packageMetadataRepo struct {
	kv kv.Store
}

func NewPackageMetadataRepo(s kv.Store) NameStore {
	return &packageMetadataRepo{kv: s}
}

func nameKey(arch, name string) string {
	return arch + "/" + name
}

// PackageFile reports a miss as ("", nil), not an error, so callers keep their
// read-through-on-miss behaviour.
func (r *packageMetadataRepo) PackageFile(arch, name string) (string, error) {
	v, err := r.kv.Get(pkgMetadataNamespace, nameKey(arch, name))
	if errors.Is(err, kv.ErrNotFound) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return string(v), nil
}

// The entry never expires (ttl 0): it is durable metadata, not a cache line.
func (r *packageMetadataRepo) StorePackageFile(arch, packageName, filePath string) error {
	return r.kv.Set(pkgMetadataNamespace, nameKey(arch, packageName), []byte(filePath), 0)
}

func (r *packageMetadataRepo) DeletePackageFileEntry(arch, packageName string) error {
	return r.kv.Delete(pkgMetadataNamespace, nameKey(arch, packageName))
}
