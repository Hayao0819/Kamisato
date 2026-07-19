package service

import (
	"fmt"
	"log/slog"
	"maps"
	"os"
	"slices"

	"github.com/Hayao0819/Kamisato/ayato/domain"
	"github.com/Hayao0819/Kamisato/ayato/repository/blob"
	"github.com/Hayao0819/Kamisato/internal/errors"
	"github.com/Hayao0819/Kamisato/pkg/pacman/reponame"
	"github.com/Hayao0819/Kamisato/pkg/raiou"
)

func (s *Service) RepoFileList(repo, arch string) ([]string, error) {
	files, err := s.pkgBinaryRepo.Files(repo, arch)
	return files, classifyRepositoryRead(err)
}

func (s *Service) Repo(repo string) (*domain.PacmanRepo, error) {
	arches, err := s.Arches(repo)
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
		return nil, classifyRepositoryRead(err)
	}

	if len(rr.Pkgs) == 0 {
		slog.Warn("no packages found in the repository", "repo", repo, "arch", arch)
	}

	infos := make([]domain.PacmanPackage, 0, len(rr.Pkgs))
	for _, pkg := range rr.Pkgs {
		infos = append(infos, pacmanPackage(pkg.PKGINFO(), pkg.Path()))
	}

	rt := domain.PacmanPkgs{
		Name:     repo,
		Arch:     arch,
		Packages: infos,
	}

	return &rt, nil
}

func (s *Service) PkgDetail(repo, arch, pkgname string) (*domain.PacmanPackage, error) {
	rr, err := s.pkgBinaryRepo.RemoteRepo(repo, arch)
	if err != nil {
		return nil, classifyRepositoryRead(err)
	}

	pkg := rr.PkgByPkgName(pkgname)
	if pkg == nil {
		return nil, fmt.Errorf(
			"%w: package %q not found in repository %s/%s",
			domain.ErrNotFound,
			pkgname,
			repo,
			arch,
		)
	}

	detail := pacmanPackage(pkg.PKGINFO(), pkg.Path())
	return &detail, nil
}

func (s *Service) RepoNames() ([]string, error) {
	return s.pkgBinaryRepo.RepoNames()
}

func (s *Service) Arches(repo string) ([]string, error) {
	arches, err := s.pkgBinaryRepo.Arches(repo)
	return arches, classifyRepositoryRead(err)
}

// classifyRepositoryRead translates storage-level absence into the service
// vocabulary. Handlers can then answer 404 without depending on a blob backend.
func classifyRepositoryRead(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, blob.ErrNotFound) || errors.Is(err, os.ErrNotExist) {
		// Preserve the backend miss for service-level callers that distinguish an
		// initialized-empty tier, while also exposing the transport-neutral domain
		// classification to handlers.
		return fmt.Errorf("%w: repository data not found: %w", domain.ErrNotFound, err)
	}
	return err
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

func pacmanPackage(info *raiou.PKGINFO, filename string) domain.PacmanPackage {
	return domain.PacmanPackage{
		PackageMetadata: packageMetadata(info),
		Filename:        filename,
	}
}

// packageMetadata converts parser-owned metadata into an immutable API
// snapshot. In particular, slices and maps must not alias repository cache
// entries that can outlive this request.
func packageMetadata(info *raiou.PKGINFO) domain.PackageMetadata {
	if info == nil {
		return domain.PackageMetadata{}
	}
	return domain.PackageMetadata{
		PkgName:     info.PkgName,
		PkgBase:     info.PkgBase,
		PkgVer:      info.PkgVer,
		PkgDesc:     info.PkgDesc,
		URL:         info.URL,
		BuildDate:   info.BuildDate,
		Packager:    info.Packager,
		Size:        info.Size,
		Arch:        info.Arch,
		License:     slices.Clone(info.License),
		Replaces:    slices.Clone(info.Replaces),
		Group:       slices.Clone(info.Group),
		Conflict:    slices.Clone(info.Conflict),
		Provides:    slices.Clone(info.Provides),
		Backup:      slices.Clone(info.Backup),
		Depend:      slices.Clone(info.Depend),
		OptDepend:   slices.Clone(info.OptDepend),
		MakeDepend:  slices.Clone(info.MakeDepend),
		CheckDepend: slices.Clone(info.CheckDepend),
		XData:       maps.Clone(info.XData),
		PkgType:     info.PkgType,
	}
}
