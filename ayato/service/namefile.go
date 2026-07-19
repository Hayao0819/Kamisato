package service

import (
	"fmt"
	"log/slog"
	"path"

	"github.com/Hayao0819/Kamisato/ayato/domain"
	"github.com/Hayao0819/Kamisato/internal/errors"
)

// resolvePackage resolves a package from the repository DB and refreshes the name cache.
func (s *Service) resolvePackage(repo, reqArch, pkgname string) (filename, storeArch string, err error) {
	var arches []string
	if reqArch != "" && reqArch != "any" {
		arches = append(arches, reqArch)
	}
	arches = append(arches, s.repoArches(repo)...)
	for _, a := range arches {
		fn, sa, dbErr := s.packageEntryFromDB(repo, a, pkgname)
		if dbErr != nil {
			continue
		}
		if storeErr := s.pkgNameRepo.StorePackageFile(repo, sa, pkgname, fn); storeErr != nil {
			slog.Warn("failed to backfill name store", "repo", repo, "arch", sa, "pkgname", pkgname, "error", storeErr.Error())
		}
		return fn, sa, nil
	}
	return "", "", fmt.Errorf("%w: package %q not found in %s", domain.ErrNotFound, pkgname, repo)
}

func (s *Service) packageEntryFromDB(repo, arch, pkgname string) (filename, pkgArch string, err error) {
	rr, err := s.pkgBinaryRepo.RemoteRepo(repo, arch)
	if err != nil {
		return "", "", errors.WrapErr(err, "fetch remote repo db")
	}

	p := rr.PkgByPkgName(pkgname)
	if p == nil {
		return "", "", fmt.Errorf("package %q not found in %s/%s db", pkgname, repo, arch)
	}

	fn := path.Base(p.Path())
	if fn == "" || fn == "." || fn == "/" {
		return "", "", fmt.Errorf("package %q has no filename in %s/%s db", pkgname, repo, arch)
	}
	return fn, p.Arch(), nil
}
