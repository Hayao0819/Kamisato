package sql

func (s *Sql) StorePackageFile(packageName, filePath string) error{
	// Convert to bytes outside the txn to reduce time spent in txn.
	key := []byte(packageName)
	val := []byte(filePath)

	err := s.db.Exec("INSERT INTO package_files (package_name, file_path) VALUES (?, ?)", key, val).Error
	if err != nil {
		return err
	}
	return nil
}
