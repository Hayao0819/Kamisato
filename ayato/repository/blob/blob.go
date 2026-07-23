// Package blob is the low-level binary object store shared by ayato's repository
// layer; it knows nothing about pacman repositories and only moves opaque files
// keyed by (repo, arch).
package blob

import (
	"os"
	"strings"
	"time"

	"github.com/Hayao0819/Kamisato/internal/errors"

	"github.com/Hayao0819/Kamisato/ayato/domain"
	"github.com/Hayao0819/Kamisato/ayato/platform"
)

//go:generate mockgen -source=blob.go -destination=../../test/mocks/blob.go -package=mocks -mock_names Store=MockBlobStore

// ErrPreconditionFailed is returned by StoreFileIfMatch when the object changed
// since it was fetched (a compare-and-swap conflict), so the caller can re-read
// the current object and retry.
var ErrPreconditionFailed = errors.New("blob: precondition failed (object changed)")

// ErrNotFound is returned by FetchFile / FetchFileWithETag when the object does
// not exist. Backends MUST return only it for absence, so a true miss is never
// confused with a transient backend error.
var ErrNotFound = errors.New("blob: not found")

// ErrSafeDeleteUnsupported means conditional orphan deletion is unavailable.
var ErrSafeDeleteUnsupported = errors.New("blob: safe conditional delete not supported")

// ValidatePathComponent rejects a repo/arch/name element that could escape its
// intended directory or key prefix. Backends compose keys by concatenating these
// components, so a "..", a "/", or an empty/"." element must be refused before it
// reaches the filesystem or object store.
func ValidatePathComponent(c string) error {
	if c == "" || c == "." || strings.ContainsRune(c, '/') || strings.ContainsRune(c, os.PathSeparator) || strings.Contains(c, "..") {
		return os.ErrNotExist
	}
	return nil
}

// FileInfo is one listed object with its last-modified time, used by orphan
// reconciliation to avoid deleting a fresh or concurrently changed object.
type FileInfo struct {
	Name         string
	LastModified time.Time
	// Version is an opaque object validator.
	Version string
}

// OrphanDeleter conditionally deletes an unchanged orphan.
type OrphanDeleter interface {
	DeleteFileIfUnchanged(repo, arch string, expected FileInfo, cutoff time.Time) (bool, error)
}

// PublicationLocker serializes publication and orphan collection for a repository.
type PublicationLocker interface {
	LockPublication(repo string) (unlock func(), err error)
}

// FileMeta is an alias for the domain-owned conditional request metadata.
type FileMeta = domain.FileMeta

// MetaFetcher is the optional capability of a Store that returns a file's
// conditional-GET metadata in a single fetch; a store without it is served full
// bodies (no validators).
type MetaFetcher interface {
	FetchFileWithMeta(repo, arch, file string) (platform.File, FileMeta, error)
}

func FetchFileWithMeta(
	store Store,
	repo, arch, file string,
) (platform.File, FileMeta, error) {
	if fetcher, ok := store.(MetaFetcher); ok {
		return fetcher.FetchFileWithMeta(repo, arch, file)
	}
	value, err := store.FetchFile(repo, arch, file)
	return value, FileMeta{}, err
}

func DeleteOrphanIfUnchanged(
	store Store,
	repo, arch string,
	expected FileInfo,
	cutoff time.Time,
) (bool, error) {
	deleter, ok := store.(OrphanDeleter)
	if !ok {
		return false, ErrSafeDeleteUnsupported
	}
	return deleter.DeleteFileIfUnchanged(repo, arch, expected, cutoff)
}

func LockPublication(store Store, repo string) (func(), error) {
	locker, ok := store.(PublicationLocker)
	if !ok {
		return func() {}, nil
	}
	return locker.LockPublication(repo)
}

// ObjectMover is an optional Store capability for migrations: raw full-key operations
// below the (repo, arch, name) API and the repo allowlist, to relocate objects
// between key layouts. Never used on the serving path.
type ObjectMover interface {
	// CopyObject copies server-side where supported (R2/S3: no download); overwriting
	// is idempotent for the immutable content a migration moves.
	CopyObject(srcKey, dstKey string) error
	ListObjects(prefix string) ([]string, error)
	// DeleteObject is idempotent: a missing key is not an error.
	DeleteObject(objKey string) error
}

// StagedIntent is one staged-upload id with its most recent object
// modification time, so GC can expire abandoned uploads.
type StagedIntent struct {
	ID      string
	ModTime time.Time
}

// StagedUploader is an optional Store capability for direct-to-storage
// uploads: presigned PUTs land in a staging prefix invisible to the serving
// key layout, and become live only after server-side validation promotes them.
type StagedUploader interface {
	PresignStagedPut(id, name string, size int64, ttl time.Duration) (string, error)
	FetchStaged(id, name string) (platform.File, error)
	// DeleteStaged removes every object under the intent; missing is not an error.
	DeleteStaged(id string) error
	// ListStagedIntents returns intent ids with their newest modification time,
	// for expiring abandoned uploads.
	ListStagedIntents() ([]StagedIntent, error)
}

// Store is pure byte/file IO and knows nothing about pacman repositories: the
// repo-DB read-modify-write lives in the domain layer (ayato/repository) that
// composes it.
type Store interface {
	StoreFile(repo, arch string, file platform.SeekFile) error
	StoreFileWithSignedURL(repo, arch, name string) (string, error)
	DeleteFile(repo, arch, file string) error
	FetchFile(repo, arch, file string) (platform.File, error)
	// FetchFileWithETag fetches a file together with an opaque version token (its
	// ETag) for an optimistic-concurrency write. The token is "" for a backend
	// without object versioning. Absence is reported like FetchFile.
	FetchFileWithETag(repo, arch, file string) (platform.File, string, error)
	// StoreFileIfMatch stores a file with compare-and-swap on its version: it
	// writes only when the live object's version still equals etag, or — when etag
	// is "" — only when the object does not yet exist (create-only). On a version
	// conflict it returns ErrPreconditionFailed.
	StoreFileIfMatch(repo, arch string, file platform.SeekFile, etag string) error
	RepoNames() ([]string, error)
	Files(repo, arch string) ([]string, error)
	// FilesWithMeta lists (repo, arch) objects with each object's last-modified
	// time, so orphan reconciliation can skip fresh or concurrently changed data.
	FilesWithMeta(repo, arch string) ([]FileInfo, error)
	Arches(repo string) ([]string, error)
}
