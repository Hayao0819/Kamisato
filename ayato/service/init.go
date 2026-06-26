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
		if err := s.initRepo(repo, false, nil); err != nil {
			return utils.WrapErr(err, fmt.Sprintf("failed to init repo %s", repo))
		}
	}
	return nil
}

// initRepo creates the repository's per-architecture databases. Arch expansion is
// a service concern (the repository layer no longer reads config): it unions the
// architectures already created on disk with the configured ones, dropping "any"
// (pacman has no os/any database). This replaces the former BinaryRepository.Init.
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
	}
	return nil
}
