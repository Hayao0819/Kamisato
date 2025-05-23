package repository

import (
	"github.com/Hayao0819/Kamisato/ayato/stream"
)

func (r *Repository) StoreFile(repo string, arch string, stream stream.IFileSeekStream) error {
	return r.pkgBinStore.StoreFile(repo, arch, stream)
}

func (r *Repository) DeleteFile(repo string, arch string, file string) error {
	return r.pkgBinStore.DeleteFile(repo, arch, file)
}

func (r *Repository) Repos() ([]string, error) {
	return r.pkgBinStore.RepoNames()
}
func (r *Repository) Files(name string, arch string) ([]string, error) {
	return r.pkgBinStore.Files(name, arch)
}

func (r *Repository) Arches(repo string) ([]string, error) {
	return r.pkgBinStore.Arches(repo)
}

func (r *Repository) FetchFile(repo string, arch string, file string) (stream.IFileStream, error) {
	return r.pkgBinStore.FetchFile(repo, arch, file)
}

func (r *Repository) RepoAdd(name string, arch string, pkg stream.IFileSeekStream, sig stream.IFileSeekStream, useSignedDB bool, gnupgDir *string) error {
	return r.pkgBinStore.RepoAdd(name, arch, pkg, sig, useSignedDB, gnupgDir)
}
func (r *Repository) RepoRemove(name string, arch string, pkg string, useSignedDB bool, gnupgDir *string) error {
	return r.pkgBinStore.RepoRemove(name, arch, pkg, useSignedDB, gnupgDir)
}
