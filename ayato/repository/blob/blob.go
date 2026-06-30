// Package blob is the low-level binary object store shared by ayato's repository
// layer; it knows nothing about pacman repositories and only moves opaque files
// keyed by (repo, arch).
package blob

import (
	"errors"

	"github.com/Hayao0819/Kamisato/ayato/stream"
)

//go:generate mockgen -source=blob.go -destination=../../test/mocks/blob.go -package=mocks -mock_names Store=MockBlobStore

// ErrPreconditionFailed is returned by StoreFileIfMatch when the object changed
// since it was fetched (a compare-and-swap conflict), so the caller can re-read
// the current object and retry.
var ErrPreconditionFailed = errors.New("blob: precondition failed (object changed)")

// ErrNotFound is returned by FetchFile / FetchFileWithETag when the object does
// not exist. Backends MUST return it (and only it) for absence, so callers can
// tell a true miss from a transient backend error and never mistake a fetch
// failure for an empty database.
var ErrNotFound = errors.New("blob: not found")

// Store is pure byte/file IO and knows nothing about pacman repositories: the
// repo-DB read-modify-write lives in the domain layer (ayato/repository) that
// composes it.
type Store interface {
	StoreFile(repo, arch string, file stream.SeekFile) error
	StoreFileWithSignedURL(repo, arch, name string) (string, error)
	DeleteFile(repo, arch, file string) error
	FetchFile(repo, arch, file string) (stream.File, error)
	// FetchFileWithETag fetches a file together with an opaque version token (its
	// ETag) for an optimistic-concurrency write. The token is "" for a backend
	// without object versioning. Absence is reported like FetchFile.
	FetchFileWithETag(repo, arch, file string) (stream.File, string, error)
	// StoreFileIfMatch stores a file with compare-and-swap on its version: it
	// writes only when the live object's version still equals etag, or — when etag
	// is "" — only when the object does not yet exist (create-only). On a version
	// conflict it returns ErrPreconditionFailed. A single-node backend (localfs)
	// has no object versioning and stores unconditionally; that is correct because
	// one process serializes its writes.
	StoreFileIfMatch(repo, arch string, file stream.SeekFile, etag string) error
	RepoNames() ([]string, error)
	Files(repo, arch string) ([]string, error)
	Arches(repo string) ([]string, error)
}
