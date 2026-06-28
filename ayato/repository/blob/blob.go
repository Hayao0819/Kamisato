// Package blob is the low-level binary object store shared by ayato's repository
// layer; it knows nothing about pacman repositories and only moves opaque files
// keyed by (repo, arch).
package blob

import (
	"github.com/Hayao0819/Kamisato/ayato/stream"
)

//go:generate mockgen -source=blob.go -destination=../../test/mocks/blob.go -package=mocks -mock_names Store=MockBlobStore

// Store is pure byte/file IO and knows nothing about pacman repositories: the
// repo-DB read-modify-write lives in the domain layer (ayato/repository) that
// composes it.
type Store interface {
	StoreFile(repo, arch string, file stream.SeekFile) error
	StoreFileWithSignedURL(repo, arch, name string) (string, error)
	DeleteFile(repo, arch, file string) error
	FetchFile(repo, arch, file string) (stream.File, error)
	RepoNames() ([]string, error)
	Files(repo, arch string) ([]string, error)
	Arches(repo string) ([]string, error)
}
