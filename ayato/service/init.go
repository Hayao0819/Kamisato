package service

import (
	"fmt"
	"log/slog"

	"github.com/Hayao0819/Kamisato/internal/utils"
)

func (s *Service) InitAll() error {
	// Fail closed at startup if the signature trust root could not be
	// established (keyring load failure, or RequireSign without a keyring).
	if s.verifierErr != nil {
		return s.verifierErr
	}
	repos := s.cfg.RepoNames()
	if len(repos) == 0 {
		slog.Warn("no repositories found in config, skipping initialization")
		return nil
	}
	slog.Debug("init all pkg repo", "repos", repos)
	for _, repo := range repos {
		slog.Debug("init pkg repo", "name", repo)
		if err := s.pkgBinaryRepo.Init(repo, false, nil); err != nil {
			return utils.WrapErr(err, fmt.Sprintf("failed to init repo %s", repo))
		}
	}
	return nil
}
