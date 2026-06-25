// Package blob is the low-level binary object store shared by ayato's repository
// layer. It deliberately knows nothing about pacman repositories, package
// metadata, or any higher-level domain: it only moves opaque files (package
// archives and DBs) in and out of a backend keyed by (repo, arch). Each backend
// (local filesystem or S3/R2-compatible storage) implements the same Store
// contract so the domain layer can ride a single abstraction.
package blob

import (
	"github.com/Hayao0819/Kamisato/ayato/stream"
)

//go:generate mockgen -source=blob.go -destination=../../test/mocks/blob.go -package=mocks -mock_names Store=MockBlobStore

// Store is the low-level backend contract for storing binaries (package files and
// DBs). It is pure byte/file IO and knows nothing about pacman repositories: the
// repo-DB read-modify-write (repo-add/repo-remove) lives in the domain layer
// (ayato/repository), which composes this contract. Implemented directly by
// localfs / s3.
type Store interface {
	StoreFile(repo, arch string, file stream.SeekFile) error
	StoreFileWithSignedURL(repo, arch, name string) (string, error)
	DeleteFile(repo, arch, file string) error
	FetchFile(repo, arch, file string) (stream.File, error)
	RepoNames() ([]string, error)
	Files(repo, arch string) ([]string, error)
	Arches(repo string) ([]string, error)
}
