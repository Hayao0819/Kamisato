package repository

func (r *Repository) GetPkgFileName(name string) (fp string, err error) {
	return r.pkgNameStore.PackageFile(name)
}

func (r *Repository) StorePkgFileName(packageName, filePath string) error {
	return r.pkgNameStore.StorePackageFile(packageName, filePath)
}

func (r *Repository) DeletePkgFileName(packageName string) error {
	return r.pkgNameStore.DeletePackageFileEntry(packageName)
}
