package service

func (s *Service) InitAll() error {
	repos, err := s.r.Repos()
	if err != nil {
		return err
	}

	for _, repo := range repos {
		if err := s.r.Init(repo, false, nil); err != nil {
			return err
		}
	}

	return nil
}
