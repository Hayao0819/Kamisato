package service

import (
	"log/slog"
)

func (s *Service) InitAll() error {
	repos, err := s.r.RepoNames()
	if err != nil {
		return err
	}
	slog.Debug("init all pkg repo", "repos", repos)

	for _, repo := range repos {
		slog.Debug("init pkg repo", "name", repo)
		if err := s.r.Init(repo, false, nil); err != nil {
			return err
		}
	}

	return nil
}
