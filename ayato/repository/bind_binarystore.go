package repository

import (
	"github.com/Hayao0819/Kamisato/ayato/stream"
)

// StoreFile saves a file to the binary store.
func (r *Repository) StoreFile(repo string, arch string, stream stream.IFileSeekStream) error {
	return r.pkgBinStore.StoreFile(repo, arch, stream)
}

// StoreFileWithSignedURL saves a file with a signed URL.
func (r *Repository) StoreFileWithSignedURL(repo string, arch string, name string) (string, error) {
	return r.pkgBinStore.StoreFileWithSignedURL(repo, arch, name)
}

// DeleteFile deletes a file from the binary store.
func (r *Repository) DeleteFile(repo string, arch string, file string) error {
	return r.pkgBinStore.DeleteFile(repo, arch, file)
}

// RepoNames returns all repository names.
func (r *Repository) RepoNames() ([]string, error) {
	return r.pkgBinStore.RepoNames()
}

// Files returns a list of files in the repository.
func (r *Repository) Files(name string, arch string) ([]string, error) {
	return r.pkgBinStore.Files(name, arch)
}

// Arches returns a list of architectures in the repository.
func (r *Repository) Arches(repo string) ([]string, error) {
	return r.pkgBinStore.Arches(repo)
}

// FetchFile retrieves a file from the binary store.
func (r *Repository) FetchFile(repo string, arch string, file string) (stream.IFileStream, error) {
	return r.pkgBinStore.FetchFile(repo, arch, file)
}

// RepoAdd adds a package to the repository.
func (r *Repository) RepoAdd(name string, arch string, pkg stream.IFileSeekStream, sig stream.IFileSeekStream, useSignedDB bool, gnupgDir *string) error {
	return r.pkgBinStore.RepoAdd(name, arch, pkg, sig, useSignedDB, gnupgDir)
}

// RepoRemove removes a package from the repository.
func (r *Repository) RepoRemove(name string, arch string, pkg string, useSignedDB bool, gnupgDir *string) error {
	return r.pkgBinStore.RepoRemove(name, arch, pkg, useSignedDB, gnupgDir)
}
