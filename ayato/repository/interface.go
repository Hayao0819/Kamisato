package repository

import (
	"github.com/BrenekH/blinky"
	"github.com/Hayao0819/Kamisato/ayato/domain"
)

type PkgNameStoreProvider blinky.PackageNameToFileProvider

type PkgBinaryStoreProvider interface {
	// StoreFile stores a file
	StoreFile(repo string, arch string, stream domain.IFileStream, useSignedDB bool, gnupgDir *string) error

	// DeleteFile deletes a file from the database
	DeleteFile(repo string, arch string, file string, useSignedDB bool, gnupgDir *string) error

	FetchFile(repo string, arch string, file string) (domain.IFileStream, error)

	// Init initializes the database.
	Init(name string, arch string, useSignedDB bool, gnupgDir *string) error
	RepoNames() ([]string, error)
	Files(repo string, arch string) ([]string, error)
	Arches(repo string) ([]string, error)
}
