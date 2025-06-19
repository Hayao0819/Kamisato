package repository

import "log/slog"

func (r *Repository) GetPkgFileName(name string) (fp string, err error) {
	return r.pkgNameStore.PackageFile(name)
}

func (r *Repository) StorePkgFileName(packageName, filePath string) error {
	err := r.pkgNameStore.StorePackageFile(packageName, filePath)
	if err != nil {
		return err
	}
	slog.Debug("store pkg file name success", "package_name", packageName, "file_path", filePath)
	return nil
}

func (r *Repository) DeletePkgFileName(packageName string) error {
	return r.pkgNameStore.DeletePackageFileEntry(packageName)
}
