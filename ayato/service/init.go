package service

import (
	"fmt"
	"log/slog"

	"github.com/Hayao0819/Kamisato/internal/errors"
)

func (s *Service) InitAll() error {
	if s.catalogErr != nil {
		return errors.WrapErr(s.catalogErr, "invalid repository catalog")
	}
	// Fail closed if the signature trust root could not be established.
	if s.verifierErr != nil {
		return s.verifierErr
	}
	// Initialize every physical repo, so each tier of a tiered repo gets its own
	// database seeded.
	repos := s.catalog.PhysicalNames()
	if len(repos) == 0 {
		slog.Warn("no repositories found in config, skipping initialization")
		return nil
	}
	slog.Debug("init all pkg repo", "repos", repos)
	for _, repo := range repos {
		slog.Debug("init pkg repo", "name", repo)
		if err := s.initRepo(repo, s.signedDB(), nil); err != nil {
			return errors.WrapErr(err, fmt.Sprintf("failed to init repo %s", repo))
		}
	}
	return nil
}

// initRepo seeds an empty db for each arch the repo serves — its declared arches plus
// any already on disk — so a fresh repo can accept an arch=any upload before any
// concrete package exists. A newly created arch is backfilled with the repo's
// existing arch=any packages.
func (s *Service) initRepo(repo string, useSignedDB bool, gnupgDir *string) error {
	for _, a := range s.repoArches(repo) {
		if err := s.ensureArchSeeded(repo, a, useSignedDB, gnupgDir); err != nil {
			return err
		}
	}
	return nil
}
