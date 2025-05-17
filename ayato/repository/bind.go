package repository

func (r *Repository) StoreFile(repo string, arch string, file string, useSignedDB bool, gnupgDir *string) error {
	return r.pkgBinStore.StoreFile(repo, arch, file, useSignedDB, gnupgDir)
}

func (r *Repository) DeleteFile(repo string, arch string, file string, useSignedDB bool, gnupgDir *string) error {
	return r.pkgBinStore.DeleteFile(repo, arch, file, useSignedDB, gnupgDir)
}

func (r *Repository) Init(useSignedDB bool, gnupgDir *string) error {
	return r.pkgBinStore.Init(useSignedDB, gnupgDir)
}
func (r *Repository) RepoNames() ([]string, error) {
	return r.pkgBinStore.RepoNames()
}
func (r *Repository) Files(name string, arch string) ([]string, error) {
	return r.pkgBinStore.Files(name, arch)
}

func (r *Repository) RepoArchs(name string) ([]string, error) {
	return r.pkgBinStore.Archs(name)
}
