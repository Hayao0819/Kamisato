package sql

func (s *Sql) DeletePackageFileEntry(packageName string) error {
	return s.db.
		Where("package_name = ?", packageName).
		Delete(&PackageFile{}).Error
}
