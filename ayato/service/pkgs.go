package service

func (s *Service) GetAllPkgs(repoName, archName string) ([]string, error) {
	pkgs, err := s.r.GetAllPkgs(repoName, archName)
	if err != nil {
		return nil, err
	}
	return pkgs, nil
}
