package repository

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math/rand/v2"
	"os"
	"path"
	"time"

	"github.com/Hayao0819/Kamisato/ayato/repository/blob"
	"github.com/Hayao0819/Kamisato/ayato/stream"
	"github.com/Hayao0819/Kamisato/internal/errwrap"
	"github.com/Hayao0819/Kamisato/pkg/pacman/repo"
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
}

func (r *binaryRepository) repoTool() repoDBTool {
	if r.tool != nil {
		return r.tool
	}
	return repo.NativeTool{}
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
func dbArtifactBases(repo string) []string {
	return []string{
		repo + ".db.tar.gz",
		repo + ".files.tar.gz",
	}
}

// writeSeekFileTo writes a SeekFile's bytes into dir under its base name.
// A nil stream is a no-op (returns "").
func writeSeekFileTo(dir string, f stream.SeekFile) (string, error) {
	if f == nil {
		return "", nil
	}
	name := path.Base(f.FileName())
	dst := path.Join(dir, name)
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return "", errwrap.WrapErr(err, "failed to seek stream")
	}
	out, err := os.Create(dst)
	if err != nil {
		return "", errwrap.WrapErr(err, "failed to create temp file")
	}
	if _, err := io.Copy(out, f); err != nil {
		_ = out.Close()
		return "", errwrap.WrapErr(err, "failed to copy stream to temp file")
	}
	if err := out.Close(); err != nil {
		return "", errwrap.WrapErr(err, "failed to close temp file")
	}
	return dst, nil
}

// storeArtifacts writes every regular file in dir back through blob.StoreFile
// under its bare name, skipping any path in skip. It stores the .tar.gz archives
// (and any .sig) but NOT the bare <repo>.db / <repo>.files copies: those are
// byte-identical to their archives and served as aliases at fetch time, so a
// single archive stays the only source of truth and no copy can trail it.
func (r *binaryRepository) storeArtifacts(repo, arch, dir string, skip map[string]struct{}) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return errwrap.WrapErr(err, "failed to read temp dir")
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if _, ok := skip[name]; ok {
			continue
		}
		if name == repo+".db" || name == repo+".files" {
			continue
		}
		fp := path.Join(dir, name)
		obj, err := stream.OpenFileWithType(fp)
		if err != nil {
			return errwrap.WrapErr(err, "failed to open artifact "+name)
		}
		// OpenFileWithType keys FileName() off the full path; re-wrap under the
		// bare name so both localfs and s3 store it as <repo>/<arch>/<name>.
		named := stream.NewFileStream(name, obj.ContentType(), obj)
		if err := r.Store.StoreFile(repo, arch, named); err != nil {
			_ = obj.Close()
			return errwrap.WrapErr(err, "failed to store artifact "+name)
		}
		_ = obj.Close()
	}
	return nil
}

// RepoAddItem is one package — and its optional detached signature — to register
// in a batch via RepoAddBatch.
type RepoAddItem struct {
	Pkg stream.SeekFile
	Sig stream.SeekFile
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

	skip := map[string]struct{}{}
	pkgPaths := make([]string, 0, len(items))
	for _, it := range items {
		pkgPath, err := writeSeekFileTo(t, it.Pkg)
		if err != nil {
			return err
		}
		if pkgPath != "" {
			skip[path.Base(pkgPath)] = struct{}{}
			pkgPaths = append(pkgPaths, pkgPath)
		}
		sigPath, err := writeSeekFileTo(t, it.Sig)
		if err != nil {
			return err
		}
		if sigPath != "" {
			skip[path.Base(sigPath)] = struct{}{}
		}
	}

	return r.mutateDB(repo, arch, t, skip, useSignedDB, func(dbPath string) error {
		if err := r.repoTool().RepoAddBatch(dbPath, pkgPaths, useSignedDB, gnupgDir); err != nil {
			slog.Error("repo db add batch", "err", err, "count", len(pkgPaths))
			return errwrap.WrapErr(err, "repo db add failed")
		}
		return nil
	})
}

// isPkgNotFound reports whether err is the tool's "no such package" sentinel. It
// lives at package scope because RepoRemove shadows the repo import with a param.
func isPkgNotFound(err error) bool {
	return errors.Is(err, repo.ErrPackageNotFound)
}

// RepoRemove removes a package from the (repo, arch) database via the same
// compare-and-swap read-modify-write as RepoAddBatch.
func (r *binaryRepository) RepoRemove(repo, arch, pkg string, useSignedDB bool, gnupgDir *string) error {
	defer r.dbMu.lock(repo + "/" + arch)()

	t, err := os.MkdirTemp("", "ayato-repodb-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(t)

	return r.mutateDB(repo, arch, t, map[string]struct{}{}, useSignedDB, func(dbPath string) error {
		if !exists(dbPath) {
			return errors.New("repository database not found")
		}
		if err := r.repoTool().RepoRemove(dbPath, pkg, useSignedDB, gnupgDir); err != nil {
			// Idempotent removal: an already-absent package is a no-op success, so a
			// retried remove after a partial failure does not error.
			if isPkgNotFound(err) {
				return nil
			}
			slog.Error("repo db remove", "err", err)
			return errwrap.WrapErr(err, "repo db remove failed")
		}
		return nil
	})
}

// InitArch ensures an empty (repo, arch) database exists WITHOUT overwriting a
// populated one. InitAll re-inits every (repo, arch) on every boot, so a blind
// write would wipe the live index on every restart/redeploy. The existence probe
// covers single-node localfs; the create-only commit (If-None-Match:*) is the
// cross-instance race net on a shared backend. Serialized per (repo, arch) via dbMu.
func (r *binaryRepository) InitArch(repo, arch string, useSignedDB bool, gnupgDir *string) error {
	defer r.dbMu.lock(repo + "/" + arch)()
	dbName := repo + ".db.tar.gz"

	if f, _, err := r.Store.FetchFileWithETag(repo, arch, dbName); err == nil {
		_ = f.Close()
		return nil // already initialized
	} else if !errors.Is(err, blob.ErrNotFound) {
		return errwrap.WrapErr(err, "repo db init: probe existing db")
	}

	slog.Debug("init pkg repo", "repo", repo, "arch", arch)
	t, err := os.MkdirTemp("", "ayato-repodb-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(t)

	if err := r.repoTool().RepoAdd(path.Join(t, dbName), "", useSignedDB, gnupgDir); err != nil {
		slog.Error("repo db init", "err", err)
		return errwrap.WrapErr(err, "repo db init failed")
	}
	// Create-only commit: a concurrent instance may have created it between the
	// probe and here; treat that as success rather than clobbering its data.
	if err := r.commitDB(repo, arch, t, map[string]struct{}{dbName: {}}, dbName, ""); err != nil {
		if errors.Is(err, blob.ErrPreconditionFailed) {
			return nil
		}
		return errwrap.WrapErr(err, "repo db init failed")
	}
	return nil
}

// mutateDB runs a (repo, arch) database read-modify-write with optimistic
// concurrency. Each attempt fetches the live DB (capturing the canonical
// <repo>.db.tar.gz version), applies mutate to the local copy, and commits with
// compare-and-swap. On a cross-writer conflict (another instance committed first)
// it re-reads and re-applies, up to maxDBAttempts. dir already holds the package
// files mutate needs (written by the caller and unchanged across attempts); only
// the DB artifacts are refetched and recomputed each attempt.
//
// The compare-and-swap is anchored on <repo>.db.tar.gz, the archive a writer loads
// to compute the next state, so the CANONICAL package set is never lost to a
// concurrent writer. The bare <repo>.db / <repo>.files pacman fetches are served as
// aliases of their archives (not stored), so they can never trail the canonical db.
//
// KNOWN LIMITATION: <repo>.files.tar.gz is still stored unconditionally after the
// canonical commit, so under a rare interleaving where one winner's store lands
// after a later winner's, the served file list can momentarily trail its db. The
// canonical db.tar.gz stays correct and the next publish reconverges. A full fix
// needs the file archive under the same atomic commit and is left as a follow-up.
// The signed-db .sig artifacts (when useSignedDB) share this window, and there it
// is stricter: a signature that trails its db.tar.gz will not verify, so pacman
// with a required SigLevel rejects the db until the next publish reconverges. The
// same atomic-commit follow-up covers it.
func (r *binaryRepository) mutateDB(repo, arch, dir string, skip map[string]struct{}, useSignedDB bool, mutate func(dbPath string) error) error {
	dbName := repo + ".db.tar.gz"
	// db.tar.gz commits via compare-and-swap, never the unconditional pass.
	skip[dbName] = struct{}{}
	dbPath := path.Join(dir, dbName)

	var lastErr error
	for attempt := range maxDBAttempts {
		if err := clearDBArtifacts(dir, repo, useSignedDB); err != nil {
			return err
		}
		etag, err := r.fetchDBArtifacts(repo, arch, dir, useSignedDB)
		if err != nil {
			return err
		}
		if err := mutate(dbPath); err != nil {
			return err // a mutation error is not a conflict; surface it
		}
		err = r.commitDB(repo, arch, dir, skip, dbName, etag)
		if err == nil {
			return nil
		}
		if !errors.Is(err, blob.ErrPreconditionFailed) {
			return err
		}
		lastErr = err
		dbConflictBackoff(attempt)
	}
	return errwrap.WrapErr(lastErr, fmt.Sprintf("repo db %s/%s: too many conflicting writers after %d attempts", repo, arch, maxDBAttempts))
}

// commitDB writes the canonical <repo>.db.tar.gz with compare-and-swap on etag
// FIRST: a conflict returns blob.ErrPreconditionFailed before any derived
// artifact is touched, so a losing writer never clobbers <repo>.files.tar.gz. On
// success the derived artifacts are written unconditionally.
func (r *binaryRepository) commitDB(repo, arch, dir string, skip map[string]struct{}, dbName, etag string) error {
	obj, err := stream.OpenFileWithType(path.Join(dir, dbName))
	if err != nil {
		return errwrap.WrapErr(err, "failed to open db archive")
	}
	named := stream.NewFileStream(dbName, obj.ContentType(), obj)
	err = r.Store.StoreFileIfMatch(repo, arch, named, etag)
	_ = obj.Close()
	if err != nil {
		return err
	}
	return r.storeArtifacts(repo, arch, dir, skip)
}

// clearDBArtifacts removes the DB artifacts a previous attempt seeded, so the next
// fetch+mutate starts from the current backend state. Package files are left
// untouched (they do not change across attempts).
func clearDBArtifacts(dir, repo string, useSignedDB bool) error {
	names := []string{repo + ".db.tar.gz", repo + ".files.tar.gz", repo + ".db", repo + ".files"}
	if useSignedDB {
		for _, b := range dbArtifactBases(repo) {
			names = append(names, b+".sig")
		}
	}
	for _, n := range names {
		if err := os.Remove(path.Join(dir, n)); err != nil && !errors.Is(err, os.ErrNotExist) {
			return errwrap.WrapErr(err, "failed to clear stale db artifact "+n)
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
// signed) for (repo, arch), returning the canonical <repo>.db.tar.gz version
// (ETag) for the compare-and-swap commit — "" when the database does not exist
// yet. Missing artifacts are tolerated.
func (r *binaryRepository) fetchDBArtifacts(repo, arch, dir string, useSignedDB bool) (string, error) {
	dbName := repo + ".db.tar.gz"
	names := dbArtifactBases(repo)
	if useSignedDB {
		for _, b := range dbArtifactBases(repo) {
			names = append(names, b+".sig")
		}
	}
	var dbETag string
	for _, name := range names {
		var (
			f    stream.File
			etag string
			err  error
		)
		if name == dbName {
			f, etag, err = r.Store.FetchFileWithETag(repo, arch, name)
		} else {
			f, err = r.Store.FetchFile(repo, arch, name)
		}
		if errors.Is(err, blob.ErrNotFound) {
			continue // genuinely absent: a fresh repo, or no .files archive yet
		}
		if err != nil {
			// A transient backend error must NOT be mistaken for "absent": that
			// would seed an empty base and overwrite the live db with a truncated
			// rebuild. Fail the attempt loudly instead.
			return "", errwrap.WrapErr(err, "failed to fetch db artifact "+name)
		}
		if name == dbName {
			dbETag = etag
		}
		dst := path.Join(dir, name)
		out, cerr := os.Create(dst)
		if cerr != nil {
			_ = f.Close()
			return "", errwrap.WrapErr(cerr, "failed to create temp db artifact")
		}
		if _, cerr := io.Copy(out, f); cerr != nil {
			_ = out.Close()
			_ = f.Close()
			return "", errwrap.WrapErr(cerr, "failed to copy db artifact")
		}
		_ = out.Close()
		_ = f.Close()
	}
	return dbETag, nil
}

func exists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}
