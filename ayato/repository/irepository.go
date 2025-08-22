package repository

import (
	"github.com/Hayao0819/Kamisato/ayato/stream"
	"github.com/Hayao0819/Kamisato/pkg/pacman/remote"
)

// IRepository defines the interface for repository operations.
// Interface definition for repository operations.
// Each method provides DB, package, file, meta store, and initialization/verification operations.
// Comments are unified in English.
//
//go:generate mockgen -source=irepository.go -destination=../test/mocks/repository.go -package=mocks
type IRepository interface {
	// Fetch DB file
	FetchDB(repoName, archName string) (stream.IFileStream, error)

	// Get list of package names
	PkgNames(repoName, archName string) ([]string, error)

	// Get remote repository
	RemoteRepo(name, arch string) (*remote.RemoteRepo, error)

	// Get list of package files
	PkgFiles(repoName, archName, pkgName string) ([]string, error)

	// File and repository operations
	StoreFile(repo string, arch string, stream stream.IFileSeekStream) error
	StoreFileWithSignedURL(repo string, arch string, name string) (string, error)
	DeleteFile(repo string, arch string, file string) error
	RepoNames() ([]string, error)
	Files(name string, arch string) ([]string, error)
	Arches(repo string) ([]string, error)
	FetchFile(repo string, arch string, file string) (stream.IFileStream, error)
	RepoAdd(name string, arch string, pkg stream.IFileSeekStream, sig stream.IFileSeekStream, useSignedDB bool, gnupgDir *string) error
	RepoRemove(name string, arch string, pkg string, useSignedDB bool, gnupgDir *string) error

	// Meta store operations
	GetPkgFileName(name string) (fp string, err error)
	StorePkgFileName(packageName, filePath string) error
	DeletePkgFileName(packageName string) error

	// Initialization and verification
	Init(name string, useSignedDB bool, gnupgDir *string) error
	VerifyPkgRepo(name string) error
}
