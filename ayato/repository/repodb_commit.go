package repository

import (
	"os"
	"path"

	"github.com/Hayao0819/Kamisato/internal/errors"

	"github.com/Hayao0819/Kamisato/ayato/platform"
	"github.com/Hayao0819/Kamisato/ayato/repository/blob"
	pacmanrepo "github.com/Hayao0819/Kamisato/pkg/pacman/repo"
)

// storeIfMatch publishes one local artifact under its bare repository name.
func (r *binaryRepository) storeIfMatch(
	repo, arch, dir, name, etag string,
) error {
	object, err := platform.OpenFileWithType(path.Join(dir, name))
	if err != nil {
		return errors.WrapErr(err, "failed to open artifact "+name)
	}
	named := platform.NewFileStream(name, object.ContentType(), object)
	storeErr := r.Store.StoreFileIfMatch(repo, arch, named, etag)
	_ = object.Close()
	return storeErr
}

// commitDB publishes canonical state first. Every later failure is marked so the
// service can reconcile instead of treating package objects as safely rollbackable.
func (r *binaryRepository) commitDB(
	repo, arch, dir string,
	etags map[string]string,
	useSignedDB bool,
) error {
	dbName := pacmanrepo.Artifacts(repo).DatabaseArchive()
	if err := r.storeIfMatch(repo, arch, dir, dbName, etags[dbName]); err != nil {
		if errors.Is(err, blob.ErrPreconditionFailed) {
			return err
		}
		return &CanonicalCommitError{Err: err}
	}
	err := r.commitDerivedArtifacts(
		repo,
		arch,
		dir,
		path.Join(dir, dbName),
		useSignedDB,
	)
	if err != nil {
		return &CanonicalCommitError{Err: err}
	}
	return nil
}

// commitDerivedAgainst rebuilds derived files only while the canonical object
// still equals the snapshot used to produce them.
func (r *binaryRepository) commitDerivedAgainst(
	repo, arch, dir, canonicalSnapshot string,
	useSignedDB bool,
) error {
	return r.commitDerivedArtifacts(repo, arch, dir, canonicalSnapshot, useSignedDB)
}

func (r *binaryRepository) commitDerivedArtifacts(
	repo, arch, dir, canonicalSnapshot string,
	useSignedDB bool,
) error {
	dbName := pacmanrepo.Artifacts(repo).DatabaseArchive()
	for _, name := range derivedArtifacts(repo, useSignedDB) {
		if !fileExists(path.Join(dir, name)) {
			continue
		}
		if err := r.commitOneDerived(
			repo,
			arch,
			dir,
			name,
			dbName,
			canonicalSnapshot,
		); err != nil {
			return err
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

func (r *binaryRepository) commitOneDerived(
	repo, arch, dir, name, dbName, canonicalSnapshot string,
) error {
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
		err = r.storeIfMatch(repo, arch, dir, name, etag)
		if err == nil {
			return nil
		}
		if !errors.Is(err, blob.ErrPreconditionFailed) {
			return err
		}
		lastErr = err
		dbConflictBackoff(attempt)
	}
	return errors.WrapErr(lastErr, "reconcile derived repo artifact "+name)
}

func (r *binaryRepository) currentObjectETag(repo, arch, name string) (string, error) {
	file, etag, err := r.Store.FetchFileWithETag(repo, arch, name)
	if errors.Is(err, blob.ErrNotFound) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return etag, file.Close()
}

// liveObjectMatchesFile verifies that a derived artifact still belongs to the
// currently published canonical database.
func (r *binaryRepository) liveObjectMatchesFile(
	repo, arch, name, localPath string,
) (bool, error) {
	local, err := os.Open(localPath)
	if err != nil {
		return false, err
	}
	localHash, hashErr := hashReader(local)
	_ = local.Close()
	if hashErr != nil {
		return false, hashErr
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
