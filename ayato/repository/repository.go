package repository

import (
	"errors"
	"fmt"
	"path"
	"strings"

	"github.com/ProtonMail/go-crypto/openpgp"

	"github.com/Hayao0819/Kamisato/ayato/domain"
	"github.com/Hayao0819/Kamisato/ayato/repository/blob"
	"github.com/Hayao0819/Kamisato/ayato/stream"
	"github.com/Hayao0819/Kamisato/internal/errwrap"
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
	RepoAddBatch(name, arch string, items []RepoAddItem, useSignedDB bool, gnupgDir *string) error
	RepoRemove(name, arch, pkg string, useSignedDB bool, gnupgDir *string) error
	InitArch(name, arch string, useSignedDB bool, gnupgDir *string) error
	FetchDB(repoName, archName string) (stream.File, error)
	FetchFileWithMeta(repo, arch, file string) (stream.File, blob.FileMeta, error)
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

// BinaryRepoOption configures a binaryRepository at construction.
type BinaryRepoOption func(*binaryRepository)

// WithSigningTool makes the repository detach-sign the .db/.files archives with
// entity when a signed database is requested. A nil entity is a no-op (the tool
// stays the unsigned Go-native writer).
func WithSigningTool(entity *openpgp.Entity) BinaryRepoOption {
	return func(r *binaryRepository) {
		if entity != nil {
			r.tool = repo.NewSigningNativeTool(entity)
		}
	}
}

func NewBinaryRepository(store blob.Store, opts ...BinaryRepoOption) BinaryRepository {
	r := &binaryRepository{Store: store}
	for _, o := range opts {
		if o != nil {
			o(r)
		}
	}
	return r
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
	if err == nil {
		return f, nil
	}
	// <repo>.db / <repo>.files are not stored; they are byte-identical aliases of
	// their .tar.gz archives, served from the single archive so no stale copy can
	// trail it. The bare name always misses, so a transient error reading the
	// archive must be surfaced, not masked as the bare-name ErrNotFound — else a
	// caller like the upload version gate reads a backend hiccup as "no such db"
	// and fails open.
	if target := dbAliasTarget(repo, file); target != "" {
		af, aerr := r.Store.FetchFile(repo, arch, target)
		if aerr == nil {
			return aliasFile{File: af, name: file}, nil
		}
		if !errors.Is(aerr, blob.ErrNotFound) {
			return nil, aerr
		}
	}
	if arch == "any" || !strings.Contains(file, "-any.pkg.tar.") {
		return f, err
	}
	if af, aerr := r.Store.FetchFile(repo, "any", file); aerr == nil {
		return af, nil
	}
	return f, err
}

// FetchFileWithMeta mirrors FetchFile's alias/any fallbacks and also returns the
// backend's conditional-GET metadata (ETag + last-modified) so the HTTP layer can
// answer If-None-Match and If-Modified-Since. The bare <repo>.db/<repo>.files
// aliases carry their archive's metadata. A backend that cannot supply metadata
// (a test store) is served without validators — a full body, never a 304.
func (r *binaryRepository) FetchFileWithMeta(repo, arch, file string) (stream.File, blob.FileMeta, error) {
	mf, ok := r.Store.(blob.MetaFetcher)
	if !ok {
		f, err := r.FetchFile(repo, arch, file)
		return f, blob.FileMeta{}, err
	}
	f, meta, err := mf.FetchFileWithMeta(repo, arch, file)
	if err == nil {
		return f, meta, nil
	}
	if target := dbAliasTarget(repo, file); target != "" {
		af, ameta, aerr := mf.FetchFileWithMeta(repo, arch, target)
		if aerr == nil {
			return aliasFile{File: af, name: file}, ameta, nil
		}
		if !errors.Is(aerr, blob.ErrNotFound) {
			return nil, blob.FileMeta{}, aerr
		}
	}
	if arch == "any" || !strings.Contains(file, "-any.pkg.tar.") {
		return f, meta, err
	}
	if af, ameta, aerr := mf.FetchFileWithMeta(repo, "any", file); aerr == nil {
		return af, ameta, nil
	}
	return f, meta, err
}

// dbAliasTarget maps a bare DB name to the archive it aliases, or "" if file is
// not an alias.
func dbAliasTarget(repo, file string) string {
	switch file {
	case repo + ".db":
		return repo + ".db.tar.gz"
	case repo + ".files":
		return repo + ".files.tar.gz"
	}
	return ""
}

// aliasFile serves an archive under the requested bare DB name so the response's
// filename matches what the client asked for.
type aliasFile struct {
	stream.File
	name string
}

func (a aliasFile) FileName() string { return a.name }

// StoreFileWithSignedURL presigns a direct-download URL for a stored object. It
// mirrors FetchFile's two alias mechanisms so the presigned key always points at
// a real object: <repo>.db / <repo>.files resolve to their .tar.gz archive, and
// an -any package (and its .sig) is stored once under "any/", so presign there.
func (r *binaryRepository) StoreFileWithSignedURL(repo, arch, name string) (string, error) {
	if target := dbAliasTarget(repo, name); target != "" {
		name = target
	} else if arch != "any" && strings.Contains(name, "-any.pkg.tar.") {
		arch = "any"
	}
	return r.Store.StoreFileWithSignedURL(repo, arch, name)
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
		return errwrap.WrapErr(err, "failed to get arches")
	}

	for _, arch := range arches {
		files, err := r.Files(name, arch)
		if err != nil {
			return errwrap.WrapErr(err, fmt.Sprintf("failed to get files for arch %s", arch))
		}

		// Only the archives are stored; <repo>.db / <repo>.files are served as
		// aliases of them, so they are not required as standalone objects.
		requiredFiles := []string{
			name + ".db.tar.gz",
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
