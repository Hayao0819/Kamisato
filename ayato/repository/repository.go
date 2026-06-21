package repository

import (
	"fmt"
	"path"

	"github.com/Hayao0819/Kamisato/ayato/stream"
	"github.com/Hayao0819/Kamisato/internal/conf"
	"github.com/Hayao0819/Kamisato/internal/utils"
	"github.com/Hayao0819/Kamisato/pkg/pacman/repo"
	"github.com/samber/lo"
)

//go:generate mockgen -source=repository.go -destination=../test/mocks/repository.go -package=mocks

// Store is the low-level backend contract for storing binaries (package files and DBs).
// Implemented directly by localfs / s3.
type Store interface {
	StoreFile(repo, arch string, file stream.SeekFile) error
	StoreFileWithSignedURL(repo, arch, name string) (string, error)
	DeleteFile(repo, arch, file string) error
	FetchFile(repo, arch, file string) (stream.File, error)
	RepoAdd(name, arch string, pkg, sig stream.SeekFile, useSignedDB bool, gnupgDir *string) error
	RepoRemove(name, arch, pkg string, useSignedDB bool, gnupgDir *string) error
	InitArch(name, arch string, useSignedDB bool, gnupgDir *string) error
	RepoNames() ([]string, error)
	Files(repo, arch string) ([]string, error)
	Arches(repo string) ([]string, error)
}

// NameStore maps package names to their stored file names (blinky-compatible).
type NameStore interface {
	PackageFile(name string) (string, error)
	StorePackageFile(packageName, filePath string) error
	DeletePackageFileEntry(packageName string) error
}

// BinaryRepository is the high-level repository the service layer depends on.
// It extends the low-level Store with pacman-specific derived operations.
type BinaryRepository interface {
	Store
	FetchDB(repoName, archName string) (stream.File, error)
	PkgNames(repoName, archName string) ([]string, error)
	RemoteRepo(name, arch string) (*repo.RemoteRepo, error)
	PkgFiles(repoName, archName, pkgName string) ([]string, error)
	// Init initializes all architectures of the repository (configured + existing).
	Init(name string, useSignedDB bool, gnupgDir *string) error
	VerifyPkgRepo(name string) error
}

// binaryRepository embeds Store and adds only the derived operations
// (low-level methods are auto-delegated via embedding, with no boilerplate).
type binaryRepository struct {
	Store
	cfg *conf.AyatoConfig
}

// NewBinaryRepository wraps a low-level Store into a BinaryRepository with derived operations.
func NewBinaryRepository(store Store, cfg *conf.AyatoConfig) BinaryRepository {
	return &binaryRepository{Store: store, cfg: cfg}
}

// FetchDB fetches the DB file for a repository and architecture.
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
