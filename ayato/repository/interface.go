package repository

import (
	"github.com/BrenekH/blinky"
	"github.com/Hayao0819/Kamisato/ayato/repository/stream"
)

type PkgNameStoreProvider blinky.PackageNameToFileProvider

type PkgBinaryStoreProvider interface {
	// StoreFile stores a file
	StoreFile(repo string, arch string, stream stream.IFileSeekStream) error

	// DeleteFile deletes a file from the database
	DeleteFile(repo string, arch string, file string) error

	// FetchFile fetches a file from the database
	FetchFile(repo string, arch string, file string) (stream.IFileStream, error)

	// RepoAdd adds a repository
	RepoAdd(name string, arch string, pkg, sig stream.IFileSeekStream, useSignedDB bool, gnupgDir *string) error

	// RepoRemove removes a repository
	RepoRemove(name string, arch string, pkg string, useSignedDB bool, gnupgDir *string) error

	// Init initializes the database.
	Init(name string, arch string, useSignedDB bool, gnupgDir *string) error

	// RepoNames returns the names of all repositories
	RepoNames() ([]string, error)

	// Files returns the files in a repository
	Files(repo string, arch string) ([]string, error)

	// Arches returns the architectures of a repository
	Arches(repo string) ([]string, error)
}
