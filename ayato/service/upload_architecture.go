package service

import (
	"fmt"
	"log/slog"

	"github.com/Hayao0819/Kamisato/internal/errors"

	"github.com/Hayao0819/Kamisato/ayato/repository"
	"github.com/Hayao0819/Kamisato/ayato/repository/blob"
	pacmanpkg "github.com/Hayao0819/Kamisato/pkg/pacman/pkg"
)

// storedArches returns concrete package directories known by the blob store.
func (s *Service) storedArches(repo string) []string {
	arches, err := s.pkgBinaryRepo.Arches(repo)
	if err != nil {
		return nil
	}
	return arches
}

func (s *Service) declaredArches(repo string) []string {
	return s.catalog.DeclaredArches(repo)
}

// repoArches is the union of configured and already stored concrete arches.
func (s *Service) repoArches(repo string) []string {
	seen := make(map[string]struct{})
	var arches []string
	for _, arch := range append(s.declaredArches(repo), s.storedArches(repo)...) {
		if arch == "" || arch == "any" {
			continue
		}
		if _, duplicate := seen[arch]; duplicate {
			continue
		}
		seen[arch] = struct{}{}
		arches = append(arches, arch)
	}
	return arches
}

// archAccepted prevents an undeclared concrete arch from silently extending an
// established repository unless allow_new_arch is enabled.
func (s *Service) archAccepted(repo, arch string) bool {
	if s.catalog.AllowsNewArch(repo) {
		return true
	}
	declared := s.declaredArches(repo)
	if containsArch(declared, arch) {
		return true
	}
	stored := s.storedArches(repo)
	if len(declared) == 0 && len(stored) == 0 {
		return true
	}
	return containsArch(stored, arch)
}

func containsArch(arches []string, target string) bool {
	for _, arch := range arches {
		if arch == target {
			return true
		}
	}
	return false
}

// targetArches maps a concrete package to itself and fans arch=any out to every
// concrete architecture served by the repository.
func (s *Service) targetArches(repo, pkgArch string) ([]string, error) {
	if pkgArch != "any" {
		return []string{pkgArch}, nil
	}
	arches := s.repoArches(repo)
	if len(arches) == 0 {
		return nil, fmt.Errorf(
			"repo %q has no architectures for an arch=any package",
			repo,
		)
	}
	return arches, nil
}

// ensureArchSeeded initializes an arch and backfills existing arch=any packages
// when the arch did not previously exist.
func (s *Service) ensureArchSeeded(
	repo, arch string,
	useSignedDB bool,
	gnupgDir *string,
) error {
	existed := containsArch(s.storedArches(repo), arch)
	if err := s.pkgBinaryRepo.InitArch(repo, arch, useSignedDB, gnupgDir); err != nil {
		return err
	}
	if useSignedDB {
		if err := s.pkgBinaryRepo.BackfillSignatures(repo, arch); err != nil {
			return err
		}
	}
	if existed {
		return nil
	}
	return s.backfillAnyInto(repo, arch, useSignedDB, gnupgDir)
}

func (s *Service) backfillAnyInto(
	repo, arch string,
	useSignedDB bool,
	gnupgDir *string,
) error {
	files, err := s.pkgBinaryRepo.Files(repo, "any")
	if err != nil {
		if errors.Is(err, blob.ErrNotFound) {
			return nil
		}
		return errors.WrapErr(err, "list any packages for backfill")
	}
	var artifacts []*spooledPackage
	defer closeSpooledPackages(artifacts)

	items := make([]repository.RepoAddItem, 0, len(files))
	for _, filename := range files {
		if !pacmanpkg.IsArchive(filename) {
			continue
		}
		artifact, err := s.spoolPackage(repo, "any", filename)
		if err != nil {
			return errors.WrapErr(err, "spool any package for backfill")
		}
		artifacts = append(artifacts, artifact)
		items = append(items, repository.RepoAddItem{
			Pkg: artifact.pkg,
			Sig: artifact.sig,
		})
	}
	if len(items) == 0 {
		return nil
	}
	slog.Info(
		"backfilling arch=any packages into new arch",
		"repo",
		repo,
		"arch",
		arch,
		"count",
		len(items),
	)
	return s.pkgBinaryRepo.RepoAddBatch(repo, arch, items, useSignedDB, gnupgDir)
}
