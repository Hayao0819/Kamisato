package repository

func (r *Repository) SetPkgFileName(name string, filePath string) error {
	return r.pkgNameDb.StorePackageFile(name, filePath)
}

func (r *Repository) GetPkgFileName(name string) (string, error) {
	return r.pkgNameDb.PackageFile(name)
}

func (r *Repository) DeletePkgFileName(name string) error {
	return r.pkgNameDb.DeletePackageFileEntry(name)
}
