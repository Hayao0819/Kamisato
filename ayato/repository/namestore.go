package repository

import (
	"errors"

	"github.com/Hayao0819/Kamisato/ayato/repository/kv"
)

// NameStore maps a package's (arch, name) to its stored file name. arch is the
// package's store arch — "any" for an arch=any package, otherwise the concrete
// architecture whose directory holds the file. Keying by (arch, name) keeps the
// same package name distinct across architectures (pacman's identity is the
// (pkgname, arch) tuple), so a per-arch lookup never returns another arch's file.
type NameStore interface {
	PackageFile(arch, name string) (string, error)
	StorePackageFile(arch, packageName, filePath string) error
	DeletePackageFileEntry(arch, packageName string) error
}

// pkgMetadataNamespace is the kv.Store namespace under which package-name ->
// file-name entries live. It isolates this domain from any other consumer
// (e.g. auth) riding the same shared kv.Store.
const pkgMetadataNamespace = "pkgfile"

// packageMetadataRepo adapts a generic kv.Store to the NameStore interface,
// preserving the package-metadata contract the service layer depends on (notably
// a miss surfacing as ("", nil) from PackageFile).
type packageMetadataRepo struct {
	kv kv.Store
}

// NewPackageMetadataRepo wraps a shared kv.Store as a NameStore scoped to the
// package-metadata namespace.
func NewPackageMetadataRepo(s kv.Store) NameStore {
	return &packageMetadataRepo{kv: s}
}

// nameKey composes the (arch, name) NameStore key so the same package name never
// collides across architectures. arch is the package's store arch ("any" or a
// concrete arch).
func nameKey(arch, name string) string {
	return arch + "/" + name
}

// PackageFile returns the stored file name for (arch, name). A miss is reported
// as ("", nil) — not an error — so callers (service.resolvePackage) keep their
// read-through-on-miss behaviour. The underlying kv.Store uniformly signals a
// miss with kv.ErrNotFound, regardless of backend.
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

// StorePackageFile records that (arch, packageName) is stored as filePath. The
// entry never expires (ttl 0): it is durable metadata, not a cache line.
func (r *packageMetadataRepo) StorePackageFile(arch, packageName, filePath string) error {
	return r.kv.Set(pkgMetadataNamespace, nameKey(arch, packageName), []byte(filePath), 0)
}

// DeletePackageFileEntry removes the (arch, packageName) metadata entry.
func (r *packageMetadataRepo) DeletePackageFileEntry(arch, packageName string) error {
	return r.kv.Delete(pkgMetadataNamespace, nameKey(arch, packageName))
}
