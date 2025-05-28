package service

// func (s *Service) GetAllPkgs(repoName, archName string) ([]string, error) {
// 	pkgs, err := s.r.PkgNames(repoName, archName)
// 	if err != nil {
// 		return nil, err
// 	}
// 	return pkgs, nil
// }

func (s *Service) PacmanRepoPkgFiles(repoName, archName, pkgName string) ([]string, error) {
	files, err := s.r.PkgFiles(repoName, archName, pkgName)
	if err != nil {
		return nil, err
	}
	return files, nil
}
