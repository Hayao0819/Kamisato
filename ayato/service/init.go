package service

import (
	"fmt"
	"log/slog"

	"github.com/Hayao0819/Kamisato/internal/errors"
)

func (s *Service) InitAll() error {
	// Fail closed if the signature trust root could not be established.
	if s.verifierErr != nil {
		return s.verifierErr
	}
	// Initialize every physical repo, so each tier of a tiered repo gets its own
	// database seeded.
	repos := s.cfg.PhysicalRepoNames()
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

// initRepo seeds an empty db for each arch the repo already has packages for. A
// fresh repo has none, so this is a no-op until the first upload creates one.
func (s *Service) initRepo(repo string, useSignedDB bool, gnupgDir *string) error {
	for _, a := range s.repoArches(repo) {
		if err := s.pkgBinaryRepo.InitArch(repo, a, useSignedDB, gnupgDir); err != nil {
			return err
		}
		// Backfill signatures for a db published before signing was enabled, so a
		// repo does not serve an unsigned db (which a required SigLevel rejects) until
		// its next mutate. Idempotent: a no-op once the db is signed.
		if useSignedDB {
			if err := s.pkgBinaryRepo.BackfillSignatures(repo, a); err != nil {
				return err
			}
		}
	}
	return nil
}
