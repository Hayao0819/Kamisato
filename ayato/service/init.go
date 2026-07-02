package service

import (
	"fmt"
	"log/slog"

	"github.com/Hayao0819/Kamisato/internal/errwrap"
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
			return errwrap.WrapErr(err, fmt.Sprintf("failed to init repo %s", repo))
		}
	}
	return nil
}

// Arch expansion lives in the service, not the repository layer: union the
// on-disk and configured arches, dropping "any" (pacman has no os/any database).
func (s *Service) initRepo(repo string, useSignedDB bool, gnupgDir *string) error {
	existing, err := s.pkgBinaryRepo.Arches(repo)
	if err != nil {
		existing = nil // a not-yet-created repo has no architecture directories
	}
	seen := make(map[string]struct{})
	arches := make([]string, 0, len(existing))
	for _, a := range append(existing, s.configuredArches(repo)...) {
		if a == "" || a == "any" {
			continue
		}
		if _, ok := seen[a]; ok {
			continue
		}
		seen[a] = struct{}{}
		arches = append(arches, a)
	}
	if len(arches) == 0 {
		return fmt.Errorf("repository %q has no architectures configured", repo)
	}
	for _, a := range arches {
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
