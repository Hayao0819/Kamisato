package repository

import (
	"errors"
	"fmt"
	"path"
	"strings"

	"github.com/Hayao0819/Kamisato/ayato/blob"
	"github.com/Hayao0819/Kamisato/ayato/kv"
	"github.com/Hayao0819/Kamisato/ayato/stream"
	"github.com/Hayao0819/Kamisato/internal/conf"
	"github.com/Hayao0819/Kamisato/internal/utils"
	"github.com/Hayao0819/Kamisato/pkg/pacman/repo"
	"github.com/samber/lo"
)

//go:generate mockgen -source=repository.go -destination=../test/mocks/repository.go -package=mocks -aux_files=github.com/Hayao0819/Kamisato/ayato/blob=../blob/blob.go

// NameStore maps package names to their stored file names (blinky-compatible).
type NameStore interface {
	PackageFile(name string) (string, error)
	StorePackageFile(packageName, filePath string) error
	DeletePackageFileEntry(packageName string) error
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

// PackageFile returns the stored file name for a package. A miss is reported as
// ("", nil) — not an error — so callers (service.resolvePackageFile) keep their
// read-through-on-miss behaviour. The underlying kv.Store uniformly signals a
// miss with kv.ErrNotFound, regardless of backend.
func (r *packageMetadataRepo) PackageFile(name string) (string, error) {
	v, err := r.kv.Get(pkgMetadataNamespace, name)
	if errors.Is(err, kv.ErrNotFound) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return string(v), nil
}

// StorePackageFile records that package packageName is stored as filePath. The
// entry never expires (ttl 0): it is durable metadata, not a cache line.
func (r *packageMetadataRepo) StorePackageFile(packageName, filePath string) error {
	return r.kv.Set(pkgMetadataNamespace, packageName, []byte(filePath), 0)
}

// DeletePackageFileEntry removes the package's metadata entry.
func (r *packageMetadataRepo) DeletePackageFileEntry(packageName string) error {
	return r.kv.Delete(pkgMetadataNamespace, packageName)
}

// BinaryRepository is the high-level repository the service layer depends on. It
// extends the pure byte/file IO of blob.Store with the pacman repo-DB domain
// operations (RepoAdd/RepoRemove/InitArch) and other derived operations. The
// blob.Store contract knows nothing about pacman; the read-modify-write of the
// repo database lives here, in the domain layer.
type BinaryRepository interface {
	blob.Store
	RepoAdd(name, arch string, pkg, sig stream.SeekFile, useSignedDB bool, gnupgDir *string) error
	RepoRemove(name, arch, pkg string, useSignedDB bool, gnupgDir *string) error
	InitArch(name, arch string, useSignedDB bool, gnupgDir *string) error
	FetchDB(repoName, archName string) (stream.File, error)
	PkgNames(repoName, archName string) ([]string, error)
	RemoteRepo(name, arch string) (*repo.RemoteRepo, error)
	PkgFiles(repoName, archName, pkgName string) ([]string, error)
	// Init initializes all architectures of the repository (configured + existing).
	Init(name string, useSignedDB bool, gnupgDir *string) error
	VerifyPkgRepo(name string) error
}

// binaryRepository embeds blob.Store (pure byte IO) and adds the pacman repo-DB
// operations and other derived operations on top. dbMu serializes the per-(repo,
// arch) database read-modify-writes (RepoAdd/RepoRemove/InitArch); it is distinct
// from any locking the underlying blob.Store performs on StoreFile/DeleteFile, so
// holding it while calling blob.StoreFile cannot deadlock.
type binaryRepository struct {
	blob.Store
	cfg  *conf.AyatoConfig
	dbMu keyedMutex
}

// NewBinaryRepository wraps a low-level blob.Store into a BinaryRepository with derived operations.
func NewBinaryRepository(store blob.Store, cfg *conf.AyatoConfig) BinaryRepository {
	return &binaryRepository{Store: store, cfg: cfg}
}

// Arches lists the repository's architectures, dropping "any". An arch=any
// package's file is kept once under the "any/" directory and registered in each
// concrete arch's db, so "any" is internal storage, never a servable arch
// (pacman fetches only os/<concrete-arch>).
func (r *binaryRepository) Arches(name string) ([]string, error) {
	arches, err := r.Store.Arches(name)
	if err != nil {
		return nil, err
	}
	return lo.Filter(arches, func(a string, _ int) bool { return a != "any" }), nil
}

// FetchFile serves a repository file. An arch=any package is stored once under
// "any/" but registered in every concrete arch's db, so a request under a
// concrete arch falls back to "any/" when the file is an any-package missing
// there. This lets os/<arch>/ serve like a normal pacman repo.
func (r *binaryRepository) FetchFile(repo, arch, file string) (stream.File, error) {
	f, err := r.Store.FetchFile(repo, arch, file)
	if err == nil || arch == "any" || !strings.Contains(file, "-any.pkg.tar.") {
		return f, err
	}
	if af, aerr := r.Store.FetchFile(repo, "any", file); aerr == nil {
		return af, nil
	}
	return f, err
}

func (r *binaryRepository) FetchDB(repoName, archName string) (stream.File, error) {
	return r.FetchFile(repoName, archName, repoName+".db")
}

// RemoteRepo parses the DB and returns a RemoteRepo.
func (r *binaryRepository) RemoteRepo(name, arch string) (*repo.RemoteRepo, error) {
	db, err := r.FetchDB(name, arch)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	rr, err := repo.RemoteRepoFromDB(name, db)
	if err != nil {
		return nil, err
	}
	if rr == nil {
		return nil, fmt.Errorf("failed to get repository")
	}
	return rr, nil
}

// PkgNames returns the pkgbase of every package in the repository.
// FIXME: opening the DB on every call is inefficient; caching or similar would help.
func (r *binaryRepository) PkgNames(repoName, archName string) ([]string, error) {
	db, err := r.FetchFile(repoName, archName, fmt.Sprintf("%s.db.tar.gz", repoName))
	if err != nil {
		return nil, err
	}
	defer db.Close()

	rr, err := repo.RemoteRepoFromDB(repoName, db)
	if err != nil {
		return nil, err
	}
	if rr == nil {
		return nil, fmt.Errorf("failed to get repository")
	}
	names := make([]string, 0, len(rr.Pkgs))
	for _, p := range rr.Pkgs {
		names = append(names, p.Base())
	}
	return names, nil
}

// PkgFiles returns the file list of a package in the repository.
// TODO: not implemented (fetching the package file list).
func (r *binaryRepository) PkgFiles(repoName, archName, pkgName string) ([]string, error) {
	db, err := r.FetchDB(repoName, archName)
	if err != nil {
		return nil, err
	}
	defer db.Close()
	if _, err := repo.RemoteRepoFromDB(repoName, db); err != nil {
		return nil, err
	}
	return nil, nil
}

// Init initializes all architectures of the repository (configured + existing).
func (r *binaryRepository) Init(name string, useSignedDB bool, gnupgDir *string) error {
	createdArches, err := r.Arches(name)
	if err != nil {
		createdArches = []string{}
	}

	repoconfig := r.cfg.Repo(name)
	if repoconfig == nil {
		return fmt.Errorf("repository %s not found in config", name)
	}

	arches := lo.Uniq(append(append([]string{}, createdArches...), repoconfig.Arches...))
	for _, arch := range arches {
		if err := r.InitArch(name, arch, useSignedDB, gnupgDir); err != nil {
			return err
		}
	}
	return nil
}

// VerifyPkgRepo verifies that each architecture has all required files.
func (r *binaryRepository) VerifyPkgRepo(name string) error {
	arches, err := r.Arches(name)
	if err != nil {
		return utils.WrapErr(err, "failed to get arches")
	}

	for _, arch := range arches {
		files, err := r.Files(name, arch)
		if err != nil {
			return utils.WrapErr(err, fmt.Sprintf("failed to get files for arch %s", arch))
		}

		requiredFiles := []string{
			name + ".db",
			name + ".db.tar.gz",
			name + ".files",
			name + ".files.tar.gz",
		}

		for _, file := range requiredFiles {
			if !lo.Contains(files, file) {
				return fmt.Errorf("%s not found in %s", file, path.Join(name, arch))
			}
		}
	}
	return nil
}
