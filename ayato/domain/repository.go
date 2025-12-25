package domain

import (
	"github.com/Hayao0819/Kamisato/ayato/stream"
	"github.com/Hayao0819/Kamisato/pkg/pacman/remote"
)

//go:generate mockgen -source=repository.go -destination=../test/mocks/repository.go -package=mocks

// IPackageBinaryRepository defines the interface for package binary and repository operations.
// This interface handles DB files, package files, repository management, and metadata operations.
type IPackageBinaryRepository interface {
	// DB Operations
	// FetchDB retrieves the DB file for the given repository and architecture
	FetchDB(repoName, archName string) (stream.IFileStream, error)

	// Package Query Operations
	// PkgNames returns all package base names in the repository
	PkgNames(repoName, archName string) ([]string, error)

	// RemoteRepo returns the remote repository object
	RemoteRepo(name, arch string) (*remote.RemoteRepo, error)

	// PkgFiles returns a list of package files in the repository
	PkgFiles(repoName, archName, pkgName string) ([]string, error)

	// File Operations
	// StoreFile saves a file to the binary store
	StoreFile(repo string, arch string, stream stream.IFileSeekStream) error

	// StoreFileWithSignedURL saves a file with a signed URL
	StoreFileWithSignedURL(repo string, arch string, name string) (string, error)

	// DeleteFile deletes a file from the binary store
	DeleteFile(repo string, arch string, file string) error

	// FetchFile retrieves a file from the binary store
	FetchFile(repo string, arch string, file string) (stream.IFileStream, error)

	// Repository Operations
	// RepoNames returns all repository names
	RepoNames() ([]string, error)

	// Files returns a list of files in the repository
	Files(name string, arch string) ([]string, error)

	// Arches returns a list of architectures in the repository
	Arches(repo string) ([]string, error)

	// RepoAdd adds a package to the repository
	RepoAdd(name string, arch string, pkg stream.IFileSeekStream, sig stream.IFileSeekStream, useSignedDB bool, gnupgDir *string) error

	// RepoRemove removes a package from the repository
	RepoRemove(name string, arch string, pkg string, useSignedDB bool, gnupgDir *string) error

	// Initialization and Verification
	// Init initializes the repository
	Init(name string, useSignedDB bool, gnupgDir *string) error

	// VerifyPkgRepo checks whether all required files exist in the repository
	VerifyPkgRepo(name string) error
}

// IPackageNameRepository defines the interface for package name management operations.
// This interface handles the mapping between package names and their file paths.
type IPackageNameRepository interface {
	// GetPkgFileName retrieves the file name from the package name
	GetPkgFileName(name string) (fp string, err error)

	// StorePkgFileName stores the package name and file path mapping
	StorePkgFileName(packageName, filePath string) error

	// DeletePkgFileName deletes the entry for the package name
	DeletePkgFileName(packageName string) error
}
