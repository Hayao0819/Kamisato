package repository

import (
	"crypto/sha256"
	stderrors "errors"
	"fmt"
	"io"
	"log/slog"
	"math/rand/v2"
	"os"
	"path"
	"time"

	"github.com/Hayao0819/Kamisato/internal/errors"

	"github.com/Hayao0819/Kamisato/ayato/repository/blob"
	"github.com/Hayao0819/Kamisato/ayato/stream"
	pacmanrepo "github.com/Hayao0819/Kamisato/pkg/pacman/repo"
)

// repoDBTool runs the pacman repo-DB mutations against a local DB path. The
// implementations live in pkg/pacman/repo: the default (repo.NativeTool) writes
// the database archives in Go, so ayato needs no repo-add/repo-remove binary and
// runs on any distribution; repo.CLITool (shelling out to repo-add) remains
// available behind the same port. Keeping it behind a port also lets
// binaryRepository be unit-tested with a fake tool.
type repoDBTool interface {
	RepoAdd(dbPath, pkgFilePath string, useSignedDB bool, gnupgDir *string) error
	RepoAddBatch(dbPath string, pkgFilePaths []string, useSignedDB bool, gnupgDir *string) error
	RepoRemove(dbPath, pkg string, useSignedDB bool, gnupgDir *string) error
	RebuildDerived(dbPath string, pkgFilePaths []string, useSignedDB bool, gnupgDir *string) error
}

func (r *binaryRepository) repoTool() repoDBTool {
	if r.tool != nil {
		return r.tool
	}
	return pacmanrepo.NativeTool{}
}

// Bounds for the optimistic-concurrency retry on a contended database. A conflict
// means another instance committed first; we re-read and re-apply after a short
// jittered backoff so contending writers do not lock-step.
const (
	maxDBAttempts         = 6
	dbConflictBackoffBase = 10 * time.Millisecond
	dbConflictBackoffCap  = 500 * time.Millisecond
)

// dbArtifactBases are the canonical repo-DB archive names repo-add/repo-remove
// read and rewrite. The matching ".db"/".files" entries are symlinks repo-add
// regenerates, so only the archives need to be seeded into the temp working dir.
func dbArtifactBases(repoName string) []string {
	return pacmanrepo.Artifacts(repoName).Archives()
}

// writeSeekFileTo writes a SeekFile's bytes into dir under its base name.
// A nil stream is a no-op (returns "").
func writeSeekFileTo(dir string, f stream.SeekFile) (string, error) {
	if f == nil {
		return "", nil
	}
	dst := path.Join(dir, path.Base(f.FileName()))
	if err := writeSeekFileToPath(dst, f); err != nil {
		return "", err
	}
	return dst, nil
}

// writeSeekFileToPath materializes f at the exact path dst, letting the caller
// name the file (e.g. "<pkg>.sig" next to its package) rather than deriving it
// from the stream's own FileName.
func writeSeekFileToPath(dst string, f stream.SeekFile) error {
	if f == nil {
		return nil
	}
	// When the caller already materialized the bytes on the same filesystem,
	// hardlink instead of re-copying them. A cross-device or any other link
	// failure falls through to the copy below.
	if d, ok := f.(stream.OnDiskFile); ok {
		if err := os.Link(d.OnDiskPath(), dst); err == nil {
			return nil
		}
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return errors.WrapErr(err, "failed to seek stream")
	}
	out, err := os.Create(dst)
	if err != nil {
		return errors.WrapErr(err, "failed to create temp file")
	}
	if _, err := io.Copy(out, f); err != nil {
		_ = out.Close()
		return errors.WrapErr(err, "failed to copy stream to temp file")
	}
	if err := out.Close(); err != nil {
		return errors.WrapErr(err, "failed to close temp file")
	}
	return nil
}

func writeReaderToPath(dst string, src io.Reader) error {
	out, err := os.Create(dst)
	if err != nil {
		return errors.WrapErr(err, "failed to create temp file")
	}
	if _, err := io.Copy(out, src); err != nil {
		_ = out.Close()
		return errors.WrapErr(err, "failed to copy stream to temp file")
	}
	if err := out.Close(); err != nil {
		return errors.WrapErr(err, "failed to close temp file")
	}
	return nil
}

// derivedArtifacts are the DB artifacts commitDB publishes alongside the canonical
// <repo>.db.tar.gz: the file database and, when signing, the detached signatures.
// The bare <repo>.db / <repo>.files (and their .sig) are NOT here — they are served
// as aliases of their archives at fetch time, so a single archive stays the only
// source of truth. Package files are the caller's StoreFile, never this path.
func derivedArtifacts(repoName string, useSignedDB bool) []string {
	artifacts := pacmanrepo.Artifacts(repoName)
	names := []string{artifacts.FilesArchive()}
	if useSignedDB {
		names = append(names, artifacts.ArchiveSignatures()...)
	}
	return names
}

// storeIfMatch commits one artifact from dir with compare-and-swap on etag,
// re-wrapping it under its bare name so both localfs and s3 store it as
// <repo>/<arch>/<name>.
func (r *binaryRepository) storeIfMatch(repo, arch, dir, name, etag string) error {
	obj, err := stream.OpenFileWithType(path.Join(dir, name))
	if err != nil {
		return errors.WrapErr(err, "failed to open artifact "+name)
	}
	named := stream.NewFileStream(name, obj.ContentType(), obj)
	err = r.Store.StoreFileIfMatch(repo, arch, named, etag)
	_ = obj.Close()
	return err
}

// RepoAddItem is one package — and its optional detached signature — to register
// in a batch via RepoAddBatch.
type RepoAddItem struct {
	Pkg                    stream.SeekFile
	Sig                    stream.SeekFile
	CheckCurrent           bool
	ExpectedName           string
	ExpectedCurrentVersion string
	ExpectedCurrentFile    string
	IntendedVersion        string
	IntendedFile           string
}

// ErrPackageChanged means the expected package state changed.
var ErrPackageChanged = errors.New("repository package changed concurrently")

// CanonicalCommitError reports a failure after a canonical DB commit.
type CanonicalCommitError struct{ Err error }

func (e *CanonicalCommitError) Error() string { return e.Err.Error() }
func (e *CanonicalCommitError) Unwrap() error { return e.Err }

func CanonicalCommitted(err error) bool {
	var committed *CanonicalCommitError
	return stderrors.As(err, &committed)
}

// RepoAdd registers a single package in the (repo, arch) database, leaving the
// package file itself to the caller's StoreFile. It is the one-item shorthand for
// RepoAddBatch.
func (r *binaryRepository) RepoAdd(repo, arch string, pkg, sig stream.SeekFile, useSignedDB bool, gnupgDir *string) error {
	return r.RepoAddBatch(repo, arch, []RepoAddItem{{Pkg: pkg, Sig: sig}}, useSignedDB, gnupgDir)
}

// RepoAddBatch registers many packages in the (repo, arch) database in a single
// read-modify-write: it fetches the live DB once, adds every package, and stores
// the result once. So N packages publish atomically — the database never appears
// with a partial set — and the blob round-trips are paid once rather than per
// package. dbMu serializes writers in this process; mutateDB's compare-and-swap
// guards concurrent instances on a shared backend.
func (r *binaryRepository) RepoAddBatch(repo, arch string, items []RepoAddItem, useSignedDB bool, gnupgDir *string) error {
	defer r.dbMu.lock(repo + "/" + arch)()

	t, err := os.MkdirTemp("", "ayato-repodb-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(t)

	pkgPaths := make([]string, 0, len(items))
	for _, it := range items {
		pkgPath, err := writeSeekFileTo(t, it.Pkg)
		if err != nil {
			return err
		}
		if pkgPath == "" {
			continue
		}
		pkgPaths = append(pkgPaths, pkgPath)
		// Write the detached signature as "<pkg>.sig" beside its package so the
		// repo tool embeds it as the desc %PGPSIG% (repo-add's --include-sigs
		// convention; NativeTool reads the same adjacent path).
		if err := writeSeekFileToPath(pkgPath+".sig", it.Sig); err != nil {
			return err
		}
	}

	return r.mutateDB(repo, arch, t, useSignedDB, func(dbPath string) error {
		if err := validateAddExpectations(dbPath, items); err != nil {
			return err
		}
		if err := r.repoTool().RepoAddBatch(dbPath, pkgPaths, useSignedDB, gnupgDir); err != nil {
			slog.Error("repo db add batch", "err", err, "count", len(pkgPaths))
			return errors.WrapErr(err, "repo db add failed")
		}
		return nil
	})
}

// isPkgNotFound reports whether err is the tool's "no such package" sentinel. It
// lives at package scope because RepoRemove shadows the repo import with a param.
func isPkgNotFound(err error) bool {
	return errors.Is(err, pacmanrepo.ErrPackageNotFound)
}

// RepoRemove removes a package from the (repo, arch) database via the same
// compare-and-swap read-modify-write as RepoAddBatch.
func (r *binaryRepository) RepoRemove(repo, arch, pkg string, useSignedDB bool, gnupgDir *string) error {
	return r.repoRemove(repo, arch, pkg, "", "", false, useSignedDB, gnupgDir)
}

func (r *binaryRepository) RepoRemoveIfMatch(repo, arch, pkg, expectedVersion, expectedFile string, useSignedDB bool, gnupgDir *string) error {
	return r.repoRemove(repo, arch, pkg, expectedVersion, expectedFile, true, useSignedDB, gnupgDir)
}

func (r *binaryRepository) repoRemove(repo, arch, pkg, expectedVersion, expectedFile string, conditional, useSignedDB bool, gnupgDir *string) error {
	defer r.dbMu.lock(repo + "/" + arch)()

	t, err := os.MkdirTemp("", "ayato-repodb-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(t)

	return r.mutateDB(repo, arch, t, useSignedDB, func(dbPath string) error {
		if !exists(dbPath) {
			if conditional {
				return ErrPackageChanged
			}
			return errors.New("repository database not found")
		}
		if conditional {
			alreadyRemoved, err := validateCurrentPackage(dbPath, pkg, expectedVersion, expectedFile)
			if err != nil {
				return err
			}
			if alreadyRemoved {
				return nil
			}
		}
		if err := r.repoTool().RepoRemove(dbPath, pkg, useSignedDB, gnupgDir); err != nil {
			// Idempotent removal: an already-absent package is a no-op success, so a
			// retried remove after a partial failure does not error.
			if isPkgNotFound(err) {
				return nil
			}
			slog.Error("repo db remove", "err", err)
			return errors.WrapErr(err, "repo db remove failed")
		}
		return nil
	})
}

func validateAddExpectations(dbPath string, items []RepoAddItem) error {
	needsCheck := false
	for _, item := range items {
		needsCheck = needsCheck || item.CheckCurrent
	}
	if !needsCheck {
		return nil
	}
	var rr *pacmanrepo.RemoteRepo
	if exists(dbPath) {
		parsed, err := pacmanrepo.RepoFromDBFile("", dbPath)
		if err != nil {
			return errors.WrapErr(err, "read repo db for conditional publish")
		}
		rr = parsed
	} else {
		rr = &pacmanrepo.RemoteRepo{}
	}
	for _, item := range items {
		if !item.CheckCurrent {
			continue
		}
		current := rr.PkgByPkgName(item.ExpectedName)
		if current != nil && item.IntendedVersion != "" && current.Version() == item.IntendedVersion && path.Base(current.Path()) == item.IntendedFile {
			continue
		}
		if item.ExpectedCurrentVersion == "" {
			if current != nil {
				return fmt.Errorf("%w: %s was added", ErrPackageChanged, item.ExpectedName)
			}
			continue
		}
		if current == nil || current.Version() != item.ExpectedCurrentVersion || path.Base(current.Path()) != item.ExpectedCurrentFile {
			return fmt.Errorf("%w: %s no longer matches %s/%s", ErrPackageChanged, item.ExpectedName, item.ExpectedCurrentVersion, item.ExpectedCurrentFile)
		}
	}
	return nil
}

func validateCurrentPackage(dbPath, name, expectedVersion, expectedFile string) (alreadyRemoved bool, err error) {
	rr, err := pacmanrepo.RepoFromDBFile("", dbPath)
	if err != nil {
		return false, errors.WrapErr(err, "read repo db for conditional remove")
	}
	current := rr.PkgByPkgName(name)
	if current == nil {
		return true, nil
	}
	if current.Version() != expectedVersion || path.Base(current.Path()) != expectedFile {
		return false, fmt.Errorf("%w: %s no longer matches %s/%s", ErrPackageChanged, name, expectedVersion, expectedFile)
	}
	return false, nil
}

// InitArch ensures an empty (repo, arch) database exists WITHOUT overwriting a
// populated one. InitAll re-inits every (repo, arch) on every boot, so a blind
// write would wipe the live index on every restart/redeploy. The existence probe
// covers single-node localfs; the create-only commit (If-None-Match:*) is the
// cross-instance race net on a shared backend. Serialized per (repo, arch) via dbMu.
func (r *binaryRepository) InitArch(repo, arch string, useSignedDB bool, gnupgDir *string) error {
	defer r.dbMu.lock(repo + "/" + arch)()
	dbName := pacmanrepo.Artifacts(repo).DatabaseArchive()

	if f, _, err := r.Store.FetchFileWithETag(repo, arch, dbName); err == nil {
		_ = f.Close()
		return r.reconcileDBLocked(repo, arch, useSignedDB, gnupgDir)
	} else if !errors.Is(err, blob.ErrNotFound) {
		return errors.WrapErr(err, "repo db init: probe existing db")
	}

	slog.Debug("init pkg repo", "repo", repo, "arch", arch)
	t, err := os.MkdirTemp("", "ayato-repodb-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(t)

	if err := r.repoTool().RepoAdd(path.Join(t, dbName), "", useSignedDB, gnupgDir); err != nil {
		slog.Error("repo db init", "err", err)
		return errors.WrapErr(err, "repo db init failed")
	}
	// Create-only commit: a concurrent instance may have created it between the
	// probe and here; treat that as success rather than clobbering its data. An
	// empty etag map makes every artifact create-only.
	if err := r.commitDB(repo, arch, t, map[string]string{}, useSignedDB); err != nil {
		if errors.Is(err, blob.ErrPreconditionFailed) {
			return nil
		}
		return errors.WrapErr(err, "repo db init failed")
	}
	return nil
}

// ReconcileDB rebuilds derived artifacts from the canonical DB.
func (r *binaryRepository) ReconcileDB(repo, arch string, useSignedDB bool, gnupgDir *string) error {
	defer r.dbMu.lock(repo + "/" + arch)()
	return r.reconcileDBLocked(repo, arch, useSignedDB, gnupgDir)
}

func (r *binaryRepository) reconcileDBLocked(repoName, arch string, useSignedDB bool, gnupgDir *string) error {
	artifacts := pacmanrepo.Artifacts(repoName)
	dbName := artifacts.DatabaseArchive()
	for attempt := range maxDBAttempts {
		t, err := os.MkdirTemp("", "ayato-repodb-reconcile-")
		if err != nil {
			return err
		}
		result := func() error {
			dbPath := path.Join(t, dbName)
			canonicalSnapshot := path.Join(t, ".canonical-"+dbName)
			canonical, _, err := r.Store.FetchFileWithETag(repoName, arch, dbName)
			if errors.Is(err, blob.ErrNotFound) {
				return nil
			}
			if err != nil {
				return errors.WrapErr(err, "reconcile repo db: fetch canonical db")
			}
			if err := writeReaderToPath(dbPath, canonical); err != nil {
				_ = canonical.Close()
				return err
			}
			if err := canonical.Close(); err != nil {
				return errors.WrapErr(err, "close canonical db")
			}
			canonicalFile, err := os.Open(dbPath)
			if err != nil {
				return err
			}
			if err := writeReaderToPath(canonicalSnapshot, canonicalFile); err != nil {
				_ = canonicalFile.Close()
				return err
			}
			if err := canonicalFile.Close(); err != nil {
				return err
			}

			filesName := artifacts.FilesArchive()
			hadFiles := false
			if currentFiles, _, ferr := r.Store.FetchFileWithETag(repoName, arch, filesName); ferr == nil {
				hadFiles = true
				if err := writeReaderToPath(path.Join(t, filesName), currentFiles); err != nil {
					_ = currentFiles.Close()
					return err
				}
				if err := currentFiles.Close(); err != nil {
					return errors.WrapErr(err, "close files database")
				}
			} else if !errors.Is(ferr, blob.ErrNotFound) {
				return errors.WrapErr(ferr, "reconcile repo db: fetch files database")
			}

			tool := r.repoTool()
			err = tool.RebuildDerived(dbPath, nil, useSignedDB, gnupgDir)
			var missing *pacmanrepo.MissingPackageFilesError
			if err != nil && !errors.As(err, &missing) && hadFiles {
				if removeErr := os.Remove(path.Join(t, filesName)); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
					return removeErr
				}
				err = tool.RebuildDerived(dbPath, nil, useSignedDB, gnupgDir)
				missing = nil
			}
			if errors.As(err, &missing) {
				pkgPaths, materializeErr := r.materializePackages(repoName, arch, t, missing.Filenames)
				if materializeErr != nil {
					return materializeErr
				}
				err = tool.RebuildDerived(dbPath, pkgPaths, useSignedDB, gnupgDir)
			}
			if err != nil {
				return errors.WrapErr(err, "rebuild derived repo artifacts")
			}
			return r.commitDerivedAgainst(repoName, arch, t, canonicalSnapshot, useSignedDB)
		}()
		_ = os.RemoveAll(t)
		if result == nil {
			return nil
		}
		if !errors.Is(result, blob.ErrPreconditionFailed) {
			return result
		}
		dbConflictBackoff(attempt)
	}
	return errors.WrapErr(blob.ErrPreconditionFailed, fmt.Sprintf("reconcile repo db %s/%s: too many conflicting writers", repoName, arch))
}

func (r *binaryRepository) materializePackages(repoName, arch, dir string, filenames []string) ([]string, error) {
	pkgDir := path.Join(dir, "packages")
	if err := os.MkdirAll(pkgDir, 0o700); err != nil {
		return nil, err
	}
	paths := make([]string, 0, len(filenames))
	seen := make(map[string]struct{}, len(filenames))
	for _, filename := range filenames {
		if path.Base(filename) != filename {
			return nil, fmt.Errorf("invalid canonical package filename %q", filename)
		}
		if _, ok := seen[filename]; ok {
			continue
		}
		seen[filename] = struct{}{}
		pkgFile, err := r.FetchFile(repoName, arch, filename)
		if err != nil {
			return nil, errors.WrapErr(err, "fetch canonical package object "+filename)
		}
		dst := path.Join(pkgDir, filename)
		if err := writeReaderToPath(dst, pkgFile); err != nil {
			_ = pkgFile.Close()
			return nil, err
		}
		if err := pkgFile.Close(); err != nil {
			return nil, errors.WrapErr(err, "close canonical package object "+filename)
		}
		paths = append(paths, dst)
	}
	return paths, nil
}

// commitDerivedAgainst publishes derived artifacts for a canonical snapshot.
func (r *binaryRepository) commitDerivedAgainst(repo, arch, dir, canonicalSnapshot string, useSignedDB bool) error {
	dbName := pacmanrepo.Artifacts(repo).DatabaseArchive()
	for _, name := range derivedArtifacts(repo, useSignedDB) {
		if !exists(path.Join(dir, name)) {
			continue
		}
		var lastErr error
		for attempt := range maxDBAttempts {
			matches, err := r.liveObjectMatchesFile(repo, arch, dbName, canonicalSnapshot)
			if err != nil {
				return err
			}
			if !matches {
				return blob.ErrPreconditionFailed
			}
			etag, err := r.currentObjectETag(repo, arch, name)
			if err != nil {
				return err
			}
			if err := r.storeIfMatch(repo, arch, dir, name, etag); err == nil {
				lastErr = nil
				break
			} else if !errors.Is(err, blob.ErrPreconditionFailed) {
				return err
			} else {
				lastErr = err
			}
			dbConflictBackoff(attempt)
		}
		if lastErr != nil {
			return errors.WrapErr(lastErr, "reconcile derived repo artifact "+name)
		}
	}
	matches, err := r.liveObjectMatchesFile(repo, arch, dbName, canonicalSnapshot)
	if err != nil {
		return err
	}
	if !matches {
		return blob.ErrPreconditionFailed
	}
	return nil
}

// BackfillSignatures regenerates the detached signatures for an existing
// (repo, arch) database whose db is already published unsigned, for a repo that
// had DB signing newly enabled. It is idempotent and a no-op when the db is absent
// or already signed; otherwise it runs an empty mutate that reloads, rewrites, and
// signs the db, committing the .sig artifacts through the same compare-and-swap
// path as a normal publish. Requires a signing tool (a signed repo has one wired).
func (r *binaryRepository) BackfillSignatures(repo, arch string) error {
	artifacts := pacmanrepo.Artifacts(repo)
	dbName := artifacts.DatabaseArchive()
	if f, err := r.Store.FetchFile(repo, arch, dbName); err != nil {
		if errors.Is(err, blob.ErrNotFound) {
			return nil // nothing published yet
		}
		return errors.WrapErr(err, "backfill: probe db")
	} else {
		_ = f.Close()
	}
	for _, name := range artifacts.ArchiveSignatures() {
		if f, err := r.Store.FetchFile(repo, arch, name); err == nil {
			_ = f.Close()
			continue
		} else if !errors.Is(err, blob.ErrNotFound) {
			return errors.WrapErr(err, "backfill: probe signature "+name)
		}
		return r.ReconcileDB(repo, arch, true, nil)
	}
	return nil
}

// mutateDB runs a (repo, arch) database read-modify-write with optimistic
// concurrency. Each attempt fetches the live DB artifacts (capturing each one's
// version), applies mutate to the local copy, and commits with compare-and-swap.
// On a cross-writer conflict on the canonical db (another instance committed first)
// it re-reads and re-applies, up to maxDBAttempts. dir already holds the package
// files mutate needs (written by the caller and unchanged across attempts); only
// the DB artifacts are refetched and recomputed each attempt.
//
// The compare-and-swap is anchored on <repo>.db.tar.gz, the archive a writer loads
// to compute the next state, so the CANONICAL package set is never lost to a
// concurrent writer. The bare <repo>.db / <repo>.files (and their .sig) pacman
// fetches are served as aliases of their archives (not stored), so they can never
// trail the canonical db.
//
// The derived artifacts — <repo>.files.tar.gz and, when signed, the detached
// signatures — are each committed with compare-and-swap on their own fetched
// version too (see commitDB), so a stale winner can no longer clobber a newer
// winner's file list or signature: a signature therefore never PERSISTENTLY trails
// the db it verifies. The only residual is the transient window inherent to two
// independently fetched objects — a client that reads <repo>.db and then <repo>.db.sig
// straddling a concurrent publish may see a mismatch and re-sync — which no plain
// object store (single-object CAS, no cross-object transaction) can close.
func (r *binaryRepository) mutateDB(repo, arch, dir string, useSignedDB bool, mutate func(dbPath string) error) error {
	dbName := pacmanrepo.Artifacts(repo).DatabaseArchive()
	dbPath := path.Join(dir, dbName)

	var lastErr error
	everCanonicalCommitted := false
	markCommitted := func(err error) error {
		if err == nil || !everCanonicalCommitted || CanonicalCommitted(err) {
			return err
		}
		return &CanonicalCommitError{Err: err}
	}
	for attempt := range maxDBAttempts {
		if err := clearDBArtifacts(dir, repo, useSignedDB); err != nil {
			return markCommitted(err)
		}
		etags, err := r.fetchDBArtifacts(repo, arch, dir, useSignedDB)
		if err != nil {
			return markCommitted(err)
		}
		if err := mutate(dbPath); err != nil {
			return markCommitted(err)
		}
		err = r.commitDB(repo, arch, dir, etags, useSignedDB)
		if err == nil {
			return nil
		}
		if CanonicalCommitted(err) {
			everCanonicalCommitted = true
		}
		if !errors.Is(err, blob.ErrPreconditionFailed) && !CanonicalCommitted(err) {
			return err
		}
		lastErr = err
		dbConflictBackoff(attempt)
	}
	return markCommitted(errors.WrapErr(lastErr, fmt.Sprintf("repo db %s/%s: too many conflicting writers after %d attempts", repo, arch, maxDBAttempts)))
}

// commitDB writes canonical state before its derived artifacts.
func (r *binaryRepository) commitDB(repo, arch, dir string, etags map[string]string, useSignedDB bool) error {
	dbName := pacmanrepo.Artifacts(repo).DatabaseArchive()
	if err := r.storeIfMatch(repo, arch, dir, dbName, etags[dbName]); err != nil {
		if errors.Is(err, blob.ErrPreconditionFailed) {
			return err
		}
		return &CanonicalCommitError{Err: err}
	}
	for _, name := range derivedArtifacts(repo, useSignedDB) {
		if !exists(path.Join(dir, name)) {
			continue
		}
		var lastErr error
		for attempt := range maxDBAttempts {
			matches, err := r.liveObjectMatchesFile(repo, arch, dbName, path.Join(dir, dbName))
			if err != nil {
				return &CanonicalCommitError{Err: err}
			}
			if !matches {
				return &CanonicalCommitError{Err: blob.ErrPreconditionFailed}
			}
			etag, err := r.currentObjectETag(repo, arch, name)
			if err != nil {
				return &CanonicalCommitError{Err: err}
			}
			err = r.storeIfMatch(repo, arch, dir, name, etag)
			if err == nil {
				lastErr = nil
				break
			}
			if !errors.Is(err, blob.ErrPreconditionFailed) {
				return &CanonicalCommitError{Err: err}
			}
			lastErr = err
			dbConflictBackoff(attempt)
		}
		if lastErr != nil {
			return &CanonicalCommitError{Err: errors.WrapErr(lastErr, "reconcile derived repo artifact "+name)}
		}
	}
	matches, err := r.liveObjectMatchesFile(repo, arch, dbName, path.Join(dir, dbName))
	if err != nil {
		return &CanonicalCommitError{Err: err}
	}
	if !matches {
		return &CanonicalCommitError{Err: blob.ErrPreconditionFailed}
	}
	return nil
}

func (r *binaryRepository) currentObjectETag(repo, arch, name string) (string, error) {
	f, etag, err := r.Store.FetchFileWithETag(repo, arch, name)
	if errors.Is(err, blob.ErrNotFound) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return etag, f.Close()
}

// liveObjectMatchesFile compares a live object with a local file.
func (r *binaryRepository) liveObjectMatchesFile(repo, arch, name, localPath string) (bool, error) {
	local, err := os.Open(localPath)
	if err != nil {
		return false, err
	}
	localHash, err := hashReader(local)
	_ = local.Close()
	if err != nil {
		return false, err
	}
	live, _, err := r.Store.FetchFileWithETag(repo, arch, name)
	if errors.Is(err, blob.ErrNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	defer live.Close()
	liveHash, err := hashReader(live)
	if err != nil {
		return false, err
	}
	return localHash == liveHash, nil
}

func hashReader(r io.Reader) ([sha256.Size]byte, error) {
	h := sha256.New()
	if _, err := io.Copy(h, r); err != nil {
		return [sha256.Size]byte{}, err
	}
	var sum [sha256.Size]byte
	copy(sum[:], h.Sum(nil))
	return sum, nil
}

// clearDBArtifacts removes the DB artifacts a previous attempt seeded, so the next
// fetch+mutate starts from the current backend state. Package files are left
// untouched (they do not change across attempts).
func clearDBArtifacts(dir, repo string, useSignedDB bool) error {
	artifacts := pacmanrepo.Artifacts(repo)
	names := append(artifacts.Archives(), artifacts.Aliases()...)
	if useSignedDB {
		names = append(names, artifacts.ArchiveSignatures()...)
		names = append(names, artifacts.AliasSignatures()...)
	}
	for _, n := range names {
		if err := os.Remove(path.Join(dir, n)); err != nil && !errors.Is(err, os.ErrNotExist) {
			return errors.WrapErr(err, "failed to clear stale db artifact "+n)
		}
	}
	return nil
}

func dbConflictBackoff(attempt int) {
	d := dbConflictBackoffBase << attempt
	if d > dbConflictBackoffCap {
		d = dbConflictBackoffCap
	}
	time.Sleep(rand.N(d)) //nolint:gosec // non-crypto retry jitter, full jitter in [0, d)
}

// fetchDBArtifacts seeds dir with the live DB archives (and their .sig when
// signed) for (repo, arch), returning a name->version (ETag) map so commitDB can
// compare-and-swap EACH artifact against the state it read. A missing artifact is
// simply absent from the map, so it commits create-only ("" etag). Missing
// artifacts are tolerated (a fresh repo, or no signatures yet).
func (r *binaryRepository) fetchDBArtifacts(repo, arch, dir string, useSignedDB bool) (map[string]string, error) {
	names := dbArtifactBases(repo)
	if useSignedDB {
		for _, b := range dbArtifactBases(repo) {
			names = append(names, b+".sig")
		}
	}
	etags := make(map[string]string, len(names))
	for _, name := range names {
		f, etag, err := r.Store.FetchFileWithETag(repo, arch, name)
		if errors.Is(err, blob.ErrNotFound) {
			continue // genuinely absent: a fresh repo, or no .files/.sig archive yet
		}
		if err != nil {
			// A transient backend error must NOT be mistaken for "absent": that
			// would seed an empty base and overwrite the live db with a truncated
			// rebuild. Fail the attempt loudly instead.
			return nil, errors.WrapErr(err, "failed to fetch db artifact "+name)
		}
		etags[name] = etag
		dst := path.Join(dir, name)
		out, cerr := os.Create(dst)
		if cerr != nil {
			_ = f.Close()
			return nil, errors.WrapErr(cerr, "failed to create temp db artifact")
		}
		if _, cerr := io.Copy(out, f); cerr != nil {
			_ = out.Close()
			_ = f.Close()
			return nil, errors.WrapErr(cerr, "failed to copy db artifact")
		}
		_ = out.Close()
		_ = f.Close()
	}
	return etags, nil
}

func exists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}
