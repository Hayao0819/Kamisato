package service

// PkgFiles returns the list of package files in the repository.
func (s *Service) PkgFiles(repoName, archName, pkgName string) ([]string, error) {
	files, err := s.pkgBinaryRepo.PkgFiles(repoName, archName, pkgName)
	if err != nil {
		return nil, err
	}
	return files, nil
}
