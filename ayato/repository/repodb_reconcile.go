package repository

import (
	"fmt"
	"os"
	"path"

	"github.com/Hayao0819/Kamisato/internal/errors"

	"github.com/Hayao0819/Kamisato/ayato/repository/blob"
	pacmanrepo "github.com/Hayao0819/Kamisato/pkg/pacman/repo"
)

// ReconcileDB rebuilds files/signature artifacts from the canonical database.
func (r *binaryRepository) ReconcileDB(
	repo, arch string,
	useSignedDB bool,
	gnupgDir *string,
) error {
	defer r.dbMu.lock(repo + "/" + arch)()
	return r.reconcileDBLocked(repo, arch, useSignedDB, gnupgDir)
}

func (r *binaryRepository) reconcileDBLocked(
	repo, arch string,
	useSignedDB bool,
	gnupgDir *string,
) error {
	for attempt := range maxDBAttempts {
		err := withRepoDBTempDir("ayato-repodb-reconcile-", func(dir string) error {
			reconciler := newDBReconciler(r, repo, arch, dir, useSignedDB, gnupgDir)
			return reconciler.run()
		})
		if err == nil {
			return nil
		}
		if !errors.Is(err, blob.ErrPreconditionFailed) {
			return err
		}
		dbConflictBackoff(attempt)
	}
	return errors.WrapErr(
		blob.ErrPreconditionFailed,
		fmt.Sprintf("reconcile repo db %s/%s: too many conflicting writers", repo, arch),
	)
}

type dbReconciler struct {
	repository        *binaryRepository
	repo              string
	arch              string
	dir               string
	useSignedDB       bool
	gnupgDir          *string
	artifacts         pacmanrepo.DatabaseArtifacts
	dbPath            string
	canonicalSnapshot string
}

func newDBReconciler(
	repository *binaryRepository,
	repo, arch, dir string,
	useSignedDB bool,
	gnupgDir *string,
) *dbReconciler {
	artifacts := pacmanrepo.Artifacts(repo)
	dbName := artifacts.DatabaseArchive()
	return &dbReconciler{
		repository:        repository,
		repo:              repo,
		arch:              arch,
		dir:               dir,
		useSignedDB:       useSignedDB,
		gnupgDir:          gnupgDir,
		artifacts:         artifacts,
		dbPath:            path.Join(dir, dbName),
		canonicalSnapshot: path.Join(dir, ".canonical-"+dbName),
	}
}

func (r *dbReconciler) run() error {
	found, err := r.fetchCanonical()
	if err != nil || !found {
		return err
	}
	hadFiles, err := r.fetchFiles()
	if err != nil {
		return err
	}
	if err := r.rebuildDerived(hadFiles); err != nil {
		return err
	}
	return r.repository.commitDerivedAgainst(
		r.repo,
		r.arch,
		r.dir,
		r.canonicalSnapshot,
		r.useSignedDB,
	)
}

func (r *dbReconciler) fetchCanonical() (bool, error) {
	dbName := r.artifacts.DatabaseArchive()
	canonical, _, err := r.repository.Store.FetchFileWithETag(r.repo, r.arch, dbName)
	if errors.Is(err, blob.ErrNotFound) {
		return false, nil
	}
	if err != nil {
		return false, errors.WrapErr(err, "reconcile repo db: fetch canonical db")
	}
	if err := writeReaderAndClose(r.dbPath, canonical); err != nil {
		return false, err
	}
	if err := copyLocalFile(r.canonicalSnapshot, r.dbPath); err != nil {
		return false, errors.WrapErr(err, "snapshot canonical db")
	}
	return true, nil
}

func (r *dbReconciler) fetchFiles() (bool, error) {
	filesName := r.artifacts.FilesArchive()
	current, _, err := r.repository.Store.FetchFileWithETag(r.repo, r.arch, filesName)
	if errors.Is(err, blob.ErrNotFound) {
		return false, nil
	}
	if err != nil {
		return false, errors.WrapErr(err, "reconcile repo db: fetch files database")
	}
	if err := writeReaderAndClose(path.Join(r.dir, filesName), current); err != nil {
		return false, errors.WrapErr(err, "materialize files database")
	}
	return true, nil
}

func (r *dbReconciler) rebuildDerived(hadFiles bool) error {
	tool := r.repository.repoTool()
	err := tool.RebuildDerived(r.dbPath, nil, r.useSignedDB, r.gnupgDir)
	var missing *pacmanrepo.MissingPackageFilesError
	if err != nil && !errors.As(err, &missing) && hadFiles {
		if removeErr := os.Remove(path.Join(r.dir, r.artifacts.FilesArchive())); removeErr != nil &&
			!errors.Is(removeErr, os.ErrNotExist) {
			return removeErr
		}
		err = tool.RebuildDerived(r.dbPath, nil, r.useSignedDB, r.gnupgDir)
	}
	if errors.As(err, &missing) {
		pkgPaths, materializeErr := r.repository.materializePackages(
			r.repo,
			r.arch,
			r.dir,
			missing.Filenames,
		)
		if materializeErr != nil {
			return materializeErr
		}
		err = tool.RebuildDerived(r.dbPath, pkgPaths, r.useSignedDB, r.gnupgDir)
	}
	if err != nil {
		return errors.WrapErr(err, "rebuild derived repo artifacts")
	}
	return nil
}
