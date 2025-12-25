package provider

import (
	"github.com/BrenekH/blinky"
	"github.com/Hayao0819/Kamisato/ayato/stream"
)

// PkgNameStoreProvider is a provider for storing package names and file names.
type PkgNameStoreProvider blinky.PackageNameToFileProvider

// PkgBinaryStoreProvider is an interface for binary file stores.
type PkgBinaryStoreProvider interface {
	// StoreFile saves a file.
	StoreFile(repo string, arch string, stream stream.IFileSeekStream) error

	// StoreFileWithSignedURL saves a file with a signed URL.
	StoreFileWithSignedURL(repo string, arch string, name string) (string, error)

	// DeleteFile deletes a file from the database.
	DeleteFile(repo string, arch string, file string) error

	// FetchFile retrieves a file from the database.
	FetchFile(repo string, arch string, file string) (stream.IFileStream, error)

	// RepoAdd adds a package to the repository.
	RepoAdd(name string, arch string, pkg, sig stream.IFileSeekStream, useSignedDB bool, gnupgDir *string) error

	// RepoRemove removes a package from the repository.
	RepoRemove(name string, arch string, pkg string, useSignedDB bool, gnupgDir *string) error

	// Init initializes the database.
	Init(name string, arch string, useSignedDB bool, gnupgDir *string) error

	// RepoNames returns all repository names.
	RepoNames() ([]string, error)

	// Files returns a list of files in the repository.
	Files(repo string, arch string) ([]string, error)

	// Arches returns a list of architectures in the repository.
	Arches(repo string) ([]string, error)
}
