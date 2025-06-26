package service

import (
	"log/slog"
	"slices"

	"github.com/Hayao0819/Kamisato/ayato/domain"
	"github.com/Hayao0819/Kamisato/pkg/raiou"
	"github.com/cockroachdb/errors"
)

func (s *Service) RepoFileList(repo, arch string) ([]string, error) {
	return s.r.Files(repo, arch)
}

func (s *Service) Repo(repo string) (*domain.PacmanRepo, error) {
	arches, err := s.r.Arches(repo)
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

func (s *Service) PkgDetail(repo, arch, pkgbase string) (*raiou.PKGINFO, error) {
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

func (s *Service) ValidateRepoName(repo string) error {
	if repo == "" {
		return nil
	}

	configuredRepos, err := s.r.RepoNames()
	if err != nil {
		return errors.Wrap(err, "failed to get repository names")
	}

	if slices.Contains(configuredRepos, repo) {
		return nil
	}

	if slices.Contains(s.cfg.RepoNames(), repo) {
		slog.Warn("repository found but failed to initialize", "repo", repo)
		return errors.New(repo + " found but failed to initialize")
	}

	return errors.New(repo + " not found in configured repositories")
}
