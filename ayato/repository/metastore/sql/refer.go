package sql

func (s *Sql) PackageFile(packageName string) (string, error) {
	var pkg PackageFile
	err := s.db.
		Where("package_name = ?", packageName).
		First(&pkg).Error

	if err != nil {
		return "", err
	}

	return pkg.FilePath, nil
}
