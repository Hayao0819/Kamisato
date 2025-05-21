package sql

func (s *Sql) DeletePackageFileEntry(packageName string) error {
	// Convert to bytes outside the txn to reduce time spent in txn.
	key := []byte(packageName)

	err := s.db.Delete(&PackageFile{}, "package_name = ?", key).Error
	if err != nil {
		return err
	}

	return nil
}
