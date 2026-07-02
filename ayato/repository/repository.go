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
	// BackfillSignatures regenerates the detached signatures for an existing
	// (repo, arch) database that was published unsigned before DB signing was
	// enabled. A no-op when the db is absent or already signed.
	BackfillSignatures(name, arch string) error
	// RebuildMerged recomputes an upstream repo's served database by merging the
	// stored upstream snapshot with the local overlay (local shadows upstream). It
	// is how a local publish/remove reaches the served view of an upstream repo.
	RebuildMerged(name, arch string, useSignedDB bool) error
	// ApplyUpstreamSnapshot records a freshly fetched upstream db/files snapshot and
	// its conditional-GET validators, then rebuilds the merged database, returning
	// the change relative to the previous snapshot.
	ApplyUpstreamSnapshot(name, arch string, dbGz, filesGz []byte, etag, lastModified string, useSignedDB bool) (repo.DBDiff, error)
	// UpstreamValidators returns the stored ETag/Last-Modified of the last synced
	// upstream snapshot (empty when none), for a conditional GET.
	UpstreamValidators(name, arch string) (etag, lastModified string, err error)
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
	// dbSigner detach-signs the merged upstream database (the repo-DB writer signs
	// the overlay itself); nil when DB signing is off.
	dbSigner *openpgp.Entity
	// upstream is the set of repos that layer an upstream database. For those, the
	// served <repo>.db/.files is the merged view (<repo>.merged.*) rather than the
	// bare overlay. Empty for a plain deployment.
	upstream map[string]bool
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
			r.dbSigner = entity
		}
	}
}

// WithUpstreamRepos marks the repos whose served database is the merge of an
// upstream database with the local overlay, so their bare <repo>.db/.files serve
// the merged view.
func WithUpstreamRepos(names []string) BinaryRepoOption {
	return func(r *binaryRepository) {
		if len(names) == 0 {
			return
		}
		r.upstream = make(map[string]bool, len(names))
		for _, n := range names {
			r.upstream[n] = true
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
	// trail it. An upstream repo serves the merged archive first, then the overlay.
	// The bare name always misses, so a transient error reading the archive must be
	// surfaced, not masked as the bare-name ErrNotFound — else a caller like the
	// upload version gate reads a backend hiccup as "no such db" and fails open.
	for _, target := range r.dbAliasTargets(repo, file) {
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
	for _, target := range r.dbAliasTargets(repo, file) {
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

// dbAliasArchive maps a bare DB name to the archive it aliases under a given base
// prefix, or "" if file is not an alias name. The detached signatures alias the
// same way: <repo>.db.sig is served from the single stored <base>.db.tar.gz.sig,
// so no copy can trail it.
func dbAliasArchive(repo, base, file string) string {
	switch file {
	case repo + ".db":
		return base + ".db.tar.gz"
	case repo + ".files":
		return base + ".files.tar.gz"
	case repo + ".db.sig":
		return base + ".db.tar.gz.sig"
	case repo + ".files.sig":
		return base + ".files.tar.gz.sig"
	}
	return ""
}

// mergedBase is the artifact prefix for a repo's merged (upstream + overlay)
// database, kept distinct from the bare overlay so RepoAddBatch's compare-and-swap
// still owns <repo>.db.tar.gz.
func mergedBase(repo string) string { return repo + ".merged" }

func (r *binaryRepository) isUpstreamRepo(repo string) bool { return r.upstream[repo] }

// dbAliasTargets lists, in order, the stored objects a bare DB name can be served
// from. For a plain repo that is just the overlay archive. For an upstream repo it
// is the merged archive first, then the overlay as a fallback before the first
// sync materializes the merged view. Returns nil when file is not a DB alias name.
func (r *binaryRepository) dbAliasTargets(repo, file string) []string {
	overlay := dbAliasArchive(repo, repo, file)
	if overlay == "" {
		return nil
	}
	if r.isUpstreamRepo(repo) {
		return []string{dbAliasArchive(repo, mergedBase(repo), file), overlay}
	}
	return []string{overlay}
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
	if overlay := dbAliasArchive(repo, repo, name); overlay != "" {
		// An upstream repo's db/files is synthesized (merged): force streaming so the
		// FetchFile merged->overlay fallback governs which artifact is served.
		if r.isUpstreamRepo(repo) {
			return "", nil
		}
		name = overlay
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

// PkgFiles returns the file list a package installs, read from the (repo, arch)
// ".files" database. A repo with no .files archive, or a package absent from it,
// is domain.ErrNotFound (the handler answers 404).
func (r *binaryRepository) PkgFiles(repoName, archName, pkgName string) ([]string, error) {
	f, err := r.FetchFile(repoName, archName, repoName+".files.tar.gz")
	if err != nil {
		if errors.Is(err, blob.ErrNotFound) {
			return nil, domain.ErrNotFound
		}
		return nil, errwrap.WrapErr(err, "failed to fetch files db")
	}
	defer f.Close()

	byName, err := repo.FilesFromDB(f)
	if err != nil {
		return nil, errwrap.WrapErr(err, "failed to parse files db")
	}
	files, ok := byName[pkgName]
	if !ok {
		return nil, fmt.Errorf("%w: package %q has no files entry in %s/%s", domain.ErrNotFound, pkgName, repoName, archName)
	}
	return files, nil
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
