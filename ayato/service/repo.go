package service

import (
	"errors"
	"log/slog"

	"github.com/Hayao0819/Kamisato/ayato/domain"
	"github.com/Hayao0819/Kamisato/pkg/raiou"
)

func (s *Service) RepoFileList(repo, arch string) ([]string, error) {
	return s.r.Files(repo, arch)
}

func (s *Service) PacmanRepo(repo string) (*domain.PacmanRepo, error) {
	arches, err := s.r.Arches(repo)
	if err != nil {
		return nil, err
	}

	pkgsgroup := make(map[string]domain.PacmanPkgs, len(arches))
	for _, arch := range arches {
		pkgs, err := s.PacmanRepoPkgs(repo, arch)
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

func (s *Service) PacmanRepoPkgs(repo, arch string) (*domain.PacmanPkgs, error) {
	rr, err := s.r.RemoteRepo(repo, arch)
	if err != nil {
		return nil, err
	}

	if len(rr.Pkgs) == 0 {
		slog.Warn("no packages found in the repository", "repo", repo, "arch", arch)
		// return nil, nil
	}

	infos := make([]raiou.PKGINFO, 0, len(rr.Pkgs))
	for _, pkg := range rr.Pkgs {
		pi := pkg.MustPKGINFO()
		infos = append(infos, *pi)
	}

	rt := domain.PacmanPkgs{
		Name:     repo,
		Arch:     arch,
		Packages: infos,
	}

	return &rt, nil
}

func (s *Service) PacmanRepoPkgDetail(repo, arch, pkgbase string) (*raiou.PKGINFO, error) {
	rr, err := s.r.RemoteRepo(repo, arch)
	if err != nil {
		return nil, err
	}

	for _, pkg := range rr.Pkgs {
		if pkg.MustPKGINFO().PkgBase == pkgbase {
			pi := pkg.MustPKGINFO()
			return pi, nil
		}
	}

	return nil, errors.New("package not found in the repository")
}

// RepoNames returns the names of all repositories.
func (s *Service) RepoNames() ([]string, error) {
	return s.r.RepoNames()
}

// Arches returns the architectures of a repository.
func (s *Service) Arches(repo string) ([]string, error) {
	return s.r.Arches(repo)
}
