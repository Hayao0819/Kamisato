package service

import (
	"fmt"
	"log/slog"

	"github.com/Hayao0819/Kamisato/internal/utils"
)

func (s *Service) InitAll() error {
	repos := s.cfg.RepoNames()
	if len(repos) == 0 {
		slog.Warn("no repositories found in config, skipping initialization")
		return nil
	}
	slog.Debug("init all pkg repo", "repos", repos)

	for _, repo := range repos {
		slog.Debug("init pkg repo", "name", repo)
		if err := s.r.Init(repo, false, nil); err != nil {
			return utils.WrapErr(err, fmt.Sprintf("failed to init repo %s", repo))
		}
	}

	return nil
}
