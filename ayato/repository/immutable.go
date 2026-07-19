package repository

import (
	"crypto/sha256"
	"fmt"
	"io"
	"time"

	"github.com/Hayao0819/Kamisato/ayato/repository/blob"
	"github.com/Hayao0819/Kamisato/ayato/stream"
	"github.com/Hayao0819/Kamisato/internal/errors"
)

// ErrImmutableObjectConflict means an immutable key contains different bytes.
var ErrImmutableObjectConflict = errors.New("repository: immutable object content conflict")

func hashSeekFile(file stream.SeekFile) ([sha256.Size]byte, error) {
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return [sha256.Size]byte{}, err
	}
	h := sha256.New()
	if _, err := io.Copy(h, file); err != nil {
		return [sha256.Size]byte{}, err
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return [sha256.Size]byte{}, err
	}
	var sum [sha256.Size]byte
	copy(sum[:], h.Sum(nil))
	return sum, nil
}

func hashFile(file stream.File) ([sha256.Size]byte, error) {
	h := sha256.New()
	if _, err := io.Copy(h, file); err != nil {
		return [sha256.Size]byte{}, err
	}
	var sum [sha256.Size]byte
	copy(sum[:], h.Sum(nil))
	return sum, nil
}

// StoreFileImmutable creates or renews a byte-identical object.
func (r *binaryRepository) StoreFileImmutable(repo, arch string, file stream.SeekFile) (bool, error) {
	want, err := hashSeekFile(file)
	if err != nil {
		return false, errors.WrapErr(err, "hash immutable object")
	}
	name := file.FileName()
	for range 3 {
		existing, etag, fetchErr := r.Store.FetchFileWithETag(repo, arch, name)
		if fetchErr == nil {
			got, hashErr := hashFile(existing)
			closeErr := existing.Close()
			if hashErr != nil {
				return false, errors.WrapErr(hashErr, "hash existing immutable object")
			}
			if closeErr != nil {
				return false, errors.WrapErr(closeErr, "close existing immutable object")
			}
			if got == want {
				if _, err := file.Seek(0, io.SeekStart); err != nil {
					return false, err
				}
				if err := r.Store.StoreFileIfMatch(repo, arch, file, etag); err == nil {
					return false, nil
				} else if errors.Is(err, blob.ErrPreconditionFailed) {
					continue
				} else {
					return false, errors.WrapErr(err, "renew immutable object")
				}
			}
			return false, fmt.Errorf("%w: %s/%s/%s", ErrImmutableObjectConflict, repo, arch, name)
		}
		if !errors.Is(fetchErr, blob.ErrNotFound) {
			return false, errors.WrapErr(fetchErr, "probe immutable object")
		}
		if _, err := file.Seek(0, io.SeekStart); err != nil {
			return false, err
		}
		if err := r.Store.StoreFileIfMatch(repo, arch, file, ""); err == nil {
			return true, nil
		} else if !errors.Is(err, blob.ErrPreconditionFailed) {
			return false, errors.WrapErr(err, "create immutable object")
		}
	}
	return false, fmt.Errorf("%w: %s/%s/%s remained contended", ErrImmutableObjectConflict, repo, arch, name)
}

func (r *binaryRepository) DeleteOrphanIfUnchanged(repo, arch string, expected blob.FileInfo, cutoff time.Time) (bool, error) {
	deleter, ok := r.Store.(blob.OrphanDeleter)
	if !ok {
		return false, blob.ErrSafeDeleteUnsupported
	}
	return deleter.DeleteFileIfUnchanged(repo, arch, expected, cutoff)
}

func (r *binaryRepository) AcquirePublicationLease(repo string) (func(), error) {
	locker, ok := r.Store.(blob.PublicationLocker)
	if !ok {
		return func() {}, nil
	}
	return locker.LockPublication(repo)
}
