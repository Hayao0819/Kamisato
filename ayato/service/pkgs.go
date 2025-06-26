package service

func (s *Service) PkgFiles(repoName, archName, pkgName string) ([]string, error) {
	files, err := s.r.PkgFiles(repoName, archName, pkgName)
	if err != nil {
		return nil, err
	}
	return files, nil
}
