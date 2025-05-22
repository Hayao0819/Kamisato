package repository

import "github.com/Hayao0819/Kamisato/ayato/domain"

func (r *Repository) StoreFile(repo string, arch string, stream domain.IFileSeekStream, useSignedDB bool, gnupgDir *string) error {
	return r.pkgBinStore.StoreFile(repo, arch, stream, useSignedDB, gnupgDir)
}

func (r *Repository) DeleteFile(repo string, arch string, file string, useSignedDB bool, gnupgDir *string) error {
	return r.pkgBinStore.DeleteFile(repo, arch, file, useSignedDB, gnupgDir)
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

func (r *Repository) FetchFile(repo string, arch string, file string) (domain.IFileStream, error) {
	return r.pkgBinStore.FetchFile(repo, arch, file)
}
