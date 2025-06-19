package service

import (
	"fmt"
	"log/slog"

	"github.com/cockroachdb/errors"
)
func (s *Service) InitAll() error {
	repos, err := s.r.RepoNames()
	if err != nil {
		return errors.Wrap(err, "failed to get repo names")
	}
	slog.Debug("init all pkg repo", "repos", repos)

	for _, repo := range repos {
		slog.Debug("init pkg repo", "name", repo)
		if err := s.r.Init(repo, false, nil); err != nil {
			return errors.Wrap(err, fmt.Sprintf("failed to init repo %s", repo))
		}
	}

	return nil
}
