// Package blob is the low-level binary object store shared by ayato's repository
// layer; it knows nothing about pacman repositories and only moves opaque files
// keyed by (repo, arch).
package blob

import (
	"os"
	"strings"
	"time"

	"github.com/Hayao0819/Kamisato/internal/errors"

	"github.com/Hayao0819/Kamisato/ayato/stream"
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

// ErrPresignUnsupported is returned by StoreFileWithSignedPutURL on a backend
// that cannot presign a direct PUT (localfs). The client falls back to the
// existing multipart upload.
var ErrPresignUnsupported = errors.New("blob: presigned upload not supported")

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

// FileInfo is one listed object with its last-modified time, used by the orphan
// reconcile to age out an object that was PUT but never finalized.
type FileInfo struct {
	Name         string
	LastModified time.Time
}

// FileMeta carries the validators the HTTP layer uses for a conditional GET: an
// opaque strong ETag (empty when the backend has no object versioning) and the
// object's last-modified time (zero when unknown).
type FileMeta struct {
	ETag         string
	LastModified time.Time
}

// MetaFetcher is the optional capability of a Store that returns a file's
// conditional-GET metadata in a single fetch; a store without it is served full
// bodies (no validators).
type MetaFetcher interface {
	FetchFileWithMeta(repo, arch, file string) (stream.File, FileMeta, error)
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

// Store is pure byte/file IO and knows nothing about pacman repositories: the
// repo-DB read-modify-write lives in the domain layer (ayato/repository) that
// composes it.
type Store interface {
	StoreFile(repo, arch string, file stream.SeekFile) error
	StoreFileWithSignedURL(repo, arch, name string) (string, error)
	// StoreFileWithSignedPutURL returns a presigned URL the client PUTs the object
	// to directly, bypassing the request-body limit in front of the server. Returns
	// ErrPresignUnsupported on a backend that cannot presign (localfs).
	StoreFileWithSignedPutURL(repo, arch, name string) (string, error)
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
	// FilesWithMeta lists (repo, arch) objects with each object's last-modified
	// time, so the orphan reconcile can skip a fresh PUT that may be mid-finalize.
	FilesWithMeta(repo, arch string) ([]FileInfo, error)
	Arches(repo string) ([]string, error)
}
