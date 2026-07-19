package repository

import (
	"log/slog"
	"path"

	"github.com/Hayao0819/Kamisato/internal/errors"

	"github.com/Hayao0819/Kamisato/ayato/repository/blob"
	pacmanrepo "github.com/Hayao0819/Kamisato/pkg/pacman/repo"
)

// InitArch creates an empty database without overwriting an existing package
// index. The initial probe avoids normal rewrites; create-only CAS handles races
// between instances.
func (r *binaryRepository) InitArch(
	repo, arch string,
	useSignedDB bool,
	gnupgDir *string,
) error {
	defer r.dbMu.lock(repo + "/" + arch)()
	dbName := pacmanrepo.Artifacts(repo).DatabaseArchive()

	if file, _, err := r.Store.FetchFileWithETag(repo, arch, dbName); err == nil {
		_ = file.Close()
		return r.reconcileDBLocked(repo, arch, useSignedDB, gnupgDir)
	} else if !errors.Is(err, blob.ErrNotFound) {
		return errors.WrapErr(err, "repo db init: probe existing db")
	}

	slog.Debug("init pkg repo", "repo", repo, "arch", arch)
	return withRepoDBTempDir("ayato-repodb-", func(dir string) error {
		if err := r.repoTool().RepoAdd(
			path.Join(dir, dbName),
			"",
			useSignedDB,
			gnupgDir,
		); err != nil {
			slog.Error("repo db init", "err", err)
			return errors.WrapErr(err, "repo db init failed")
		}
		// No ETags means create-only for every artifact.
		if err := r.commitDB(repo, arch, dir, map[string]string{}, useSignedDB); err != nil {
			if errors.Is(err, blob.ErrPreconditionFailed) {
				return nil
			}
			return errors.WrapErr(err, "repo db init failed")
		}
		return nil
	})
}

// BackfillSignatures creates detached database signatures after signing has been
// enabled for an existing repository. It is idempotent.
func (r *binaryRepository) BackfillSignatures(repo, arch string) error {
	artifacts := pacmanrepo.Artifacts(repo)
	if file, err := r.Store.FetchFile(repo, arch, artifacts.DatabaseArchive()); err != nil {
		if errors.Is(err, blob.ErrNotFound) {
			return nil
		}
		return errors.WrapErr(err, "backfill: probe db")
	} else {
		_ = file.Close()
	}
	for _, name := range artifacts.ArchiveSignatures() {
		if file, err := r.Store.FetchFile(repo, arch, name); err == nil {
			_ = file.Close()
			continue
		} else if !errors.Is(err, blob.ErrNotFound) {
			return errors.WrapErr(err, "backfill: probe signature "+name)
		}
		return r.ReconcileDB(repo, arch, true, nil)
	}
	return nil
}
