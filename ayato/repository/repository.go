package repository

import (
	"fmt"
	"path"
	"strings"

	"github.com/Hayao0819/Kamisato/ayato/domain"
	"github.com/Hayao0819/Kamisato/ayato/repository/blob"
	"github.com/Hayao0819/Kamisato/ayato/stream"
	"github.com/Hayao0819/Kamisato/internal/utils"
	"github.com/Hayao0819/Kamisato/pkg/pacman/repo"
	"github.com/samber/lo"
)

//go:generate mockgen -source=repository.go -destination=../test/mocks/repository.go -package=mocks -aux_files=github.com/Hayao0819/Kamisato/ayato/repository/blob=blob/blob.go

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
	VerifyPkgRepo(name string) error
}

// binaryRepository embeds blob.Store (pure byte IO) and adds the pacman repo-DB
// operations and other derived operations on top. dbMu serializes the per-(repo,
// arch) database read-modify-writes (RepoAdd/RepoRemove/InitArch); it is distinct
// from any locking the underlying blob.Store performs on StoreFile/DeleteFile, so
// holding it while calling blob.StoreFile cannot deadlock.
type binaryRepository struct {
	blob.Store
	dbMu keyedMutex
	// tool runs the repo-DB mutations; nil defaults to the Go-native writer
	// (repo.NativeTool). Injected (e.g. a fake) in tests, or set to repo.CLITool
	// to shell out to repo-add instead.
	tool repoDBTool
}

func NewBinaryRepository(store blob.Store) BinaryRepository {
	return &binaryRepository{Store: store}
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

// Not implemented: the .files database is not parsed yet, so this reports
// domain.ErrNotImplemented rather than a misleading empty list (the handler
// answers 501).
func (r *binaryRepository) PkgFiles(repoName, archName, pkgName string) ([]string, error) {
	return nil, domain.ErrNotImplemented
}

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
