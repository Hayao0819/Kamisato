package service

import (
	"fmt"
	"log/slog"
	"slices"

	"github.com/Hayao0819/Kamisato/ayato/domain"
	"github.com/Hayao0819/Kamisato/internal/errors"
	"github.com/Hayao0819/Kamisato/pkg/pacman/reponame"
)

func (s *Service) RepoFileList(repo, arch string) ([]string, error) {
	return s.pkgBinaryRepo.Files(repo, arch)
}

func (s *Service) Repo(repo string) (*domain.PacmanRepo, error) {
	arches, err := s.pkgBinaryRepo.Arches(repo)
	if err != nil {
		return nil, err
	}

	pkgsgroup := make(map[string]domain.PacmanPkgs, len(arches))
	for _, arch := range arches {
		pkgs, err := s.Pkgs(repo, arch)
		if err != nil {
			return nil, err
		}
		pkgsgroup[arch] = *pkgs
	}

	rt := domain.PacmanRepo{
		Name:     repo,
		Arches:   arches,
		Packages: pkgsgroup,
	}
	return &rt, nil
}

func (s *Service) Pkgs(repo, arch string) (*domain.PacmanPkgs, error) {
	rr, err := s.pkgBinaryRepo.RemoteRepo(repo, arch)
	if err != nil {
		return nil, err
	}

	if len(rr.Pkgs) == 0 {
		slog.Warn("no packages found in the repository", "repo", repo, "arch", arch)
	}

	infos := make([]domain.PacmanPackage, 0, len(rr.Pkgs))
	for _, pkg := range rr.Pkgs {
		pi := pkg.PKGINFO()
		infos = append(infos, domain.PacmanPackage{
			PKGINFO:  *pi,
			Filename: pkg.Path(),
		})
	}

	rt := domain.PacmanPkgs{
		Name:     repo,
		Arch:     arch,
		Packages: infos,
	}

	return &rt, nil
}

func (s *Service) PkgDetail(repo, arch, pkgbase string) (*domain.PacmanPackage, error) {
	rr, err := s.pkgBinaryRepo.RemoteRepo(repo, arch)
	if err != nil {
		return nil, err
	}

	for _, pkg := range rr.Pkgs {
		if pkg.PKGINFO().PkgBase == pkgbase {
			return &domain.PacmanPackage{
				PKGINFO:  *pkg.PKGINFO(),
				Filename: pkg.Path(),
			}, nil
		}
	}

	return nil, errors.NewErr("package not found in the repository")
}

func (s *Service) RepoNames() ([]string, error) {
	return s.pkgBinaryRepo.RepoNames()
}

func (s *Service) Arches(repo string) ([]string, error) {
	return s.pkgBinaryRepo.Arches(repo)
}

func (s *Service) ValidateRepoName(repo string) error {
	if err := reponame.Validate(repo); err != nil {
		return fmt.Errorf("%w: %v", domain.ErrInvalid, err)
	}
	if _, configured := s.catalog.Resolve(repo); !configured {
		return fmt.Errorf("%w: %s not found in configured repositories", domain.ErrNotFound, repo)
	}
	initializedRepos, err := s.pkgBinaryRepo.RepoNames()
	if err != nil {
		return errors.WrapErr(err, "failed to get repository names")
	}
	if slices.Contains(initializedRepos, repo) {
		return nil
	}
	slog.Warn("repository found but failed to initialize", "repo", repo)
	return errors.NewErr(repo + " found but failed to initialize")
}
