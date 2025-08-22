package repository

import "log/slog"

// GetPkgFileName retrieves the file name from the package name.
func (r *Repository) GetPkgFileName(name string) (fp string, err error) {
	return r.pkgNameStore.PackageFile(name)
}

// StorePkgFileName stores the package name and file path in the store.
func (r *Repository) StorePkgFileName(packageName, filePath string) error {
	err := r.pkgNameStore.StorePackageFile(packageName, filePath)
	if err != nil {
		return err
	}
	slog.Debug("store pkg file name success", "package_name", packageName, "file_path", filePath)
	return nil
}

// DeletePkgFileName deletes the entry for the package name from the store.
func (r *Repository) DeletePkgFileName(packageName string) error {
	return r.pkgNameStore.DeletePackageFileEntry(packageName)
}
