package repository

import (
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

// derivedArtifacts are the DB artifacts commitDB publishes alongside the canonical
// <repo>.db.tar.gz: the file database and, when signing, the detached signatures.
// The bare <repo>.db / <repo>.files (and their .sig) are NOT here — they are served
// as aliases of their archives at fetch time, so a single archive stays the only
// source of truth. Package files are the caller's StoreFile, never this path.
func derivedArtifacts(repo string, useSignedDB bool) []string {
	names := []string{repo + ".files.tar.gz"}
	if useSignedDB {
		names = append(names, repo+".db.tar.gz.sig", repo+".files.tar.gz.sig")
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

	return r.mutateDB(repo, arch, t, useSignedDB, func(dbPath string) error {
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
			return errors.WrapErr(err, "repo db remove failed")
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

// BackfillSignatures regenerates the detached signatures for an existing
// (repo, arch) database whose db is already published unsigned, for a repo that
// had DB signing newly enabled. It is idempotent and a no-op when the db is absent
// or already signed; otherwise it runs an empty mutate that reloads, rewrites, and
// signs the db, committing the .sig artifacts through the same compare-and-swap
// path as a normal publish. Requires a signing tool (a signed repo has one wired).
func (r *binaryRepository) BackfillSignatures(repo, arch string) error {
	dbName := repo + ".db.tar.gz"
	if f, err := r.Store.FetchFile(repo, arch, dbName); err != nil {
		if errors.Is(err, blob.ErrNotFound) {
			return nil // nothing published yet
		}
		return errors.WrapErr(err, "backfill: probe db")
	} else {
		_ = f.Close()
	}
	if f, err := r.Store.FetchFile(repo, arch, dbName+".sig"); err == nil {
		_ = f.Close()
		return nil // already signed
	} else if !errors.Is(err, blob.ErrNotFound) {
		return errors.WrapErr(err, "backfill: probe db signature")
	}
	return r.RepoAddBatch(repo, arch, nil, true, nil)
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
	dbName := repo + ".db.tar.gz"
	dbPath := path.Join(dir, dbName)

	var lastErr error
	for attempt := range maxDBAttempts {
		if err := clearDBArtifacts(dir, repo, useSignedDB); err != nil {
			return err
		}
		etags, err := r.fetchDBArtifacts(repo, arch, dir, useSignedDB)
		if err != nil {
			return err
		}
		if err := mutate(dbPath); err != nil {
			return err // a mutation error is not a conflict; surface it
		}
		err = r.commitDB(repo, arch, dir, etags, useSignedDB)
		if err == nil {
			return nil
		}
		if !errors.Is(err, blob.ErrPreconditionFailed) {
			return err
		}
		lastErr = err
		dbConflictBackoff(attempt)
	}
	return errors.WrapErr(lastErr, fmt.Sprintf("repo db %s/%s: too many conflicting writers after %d attempts", repo, arch, maxDBAttempts))
}

// commitDB publishes the mutation. The canonical <repo>.db.tar.gz commits FIRST
// with compare-and-swap on its fetched version: a conflict returns
// blob.ErrPreconditionFailed before any derived artifact is touched, so a losing
// writer never clobbers a newer winner's derived artifacts. Each derived artifact
// (the file db and the detached signatures) then commits with compare-and-swap on
// its OWN fetched version, and a precondition failure there is success: it means a
// newer db winner already published a newer artifact, so the live set stays
// consistent with the live db and a signature never persistently trails it.
func (r *binaryRepository) commitDB(repo, arch, dir string, etags map[string]string, useSignedDB bool) error {
	dbName := repo + ".db.tar.gz"
	if err := r.storeIfMatch(repo, arch, dir, dbName, etags[dbName]); err != nil {
		return err // a db conflict propagates so mutateDB retries
	}
	for _, name := range derivedArtifacts(repo, useSignedDB) {
		if !exists(path.Join(dir, name)) {
			continue
		}
		if err := r.storeIfMatch(repo, arch, dir, name, etags[name]); err != nil {
			if errors.Is(err, blob.ErrPreconditionFailed) {
				continue // a newer winner already advanced this artifact
			}
			return err
		}
	}
	return nil
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
