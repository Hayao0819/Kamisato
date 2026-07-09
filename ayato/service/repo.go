package service

import (
	"log/slog"
	"slices"

	"github.com/Hayao0819/Kamisato/ayato/domain"
	"github.com/Hayao0819/Kamisato/internal/errors"
	"github.com/Hayao0819/Kamisato/pkg/raiou"
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

	infos := make([]raiou.PKGINFO, 0, len(rr.Pkgs))
	for _, pkg := range rr.Pkgs {
		pi := pkg.PKGINFO()
		infos = append(infos, *pi)
	}

	rt := domain.PacmanPkgs{
		Name:     repo,
		Arch:     arch,
		Packages: infos,
	}

	return &rt, nil
}

func (s *Service) PkgDetail(repo, arch, pkgbase string) (*raiou.PKGINFO, error) {
	rr, err := s.pkgBinaryRepo.RemoteRepo(repo, arch)
	if err != nil {
		return nil, err
	}

	for _, pkg := range rr.Pkgs {
		if pkg.PKGINFO().PkgBase == pkgbase {
			pi := pkg.PKGINFO()
			return pi, nil
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
	if repo == "" {
		return nil
	}
	configuredRepos, err := s.pkgBinaryRepo.RepoNames()
	if err != nil {
		return errors.WrapErr(err, "failed to get repository names")
	}
	if slices.Contains(configuredRepos, repo) {
		return nil
	}
	if slices.Contains(s.cfg.PhysicalRepoNames(), repo) {
		slog.Warn("repository found but failed to initialize", "repo", repo)
		return errors.NewErr(repo + " found but failed to initialize")
	}
	return errors.NewErr(repo + " not found in configured repositories")
}
