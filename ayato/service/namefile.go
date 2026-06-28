package service

import (
	"fmt"
	"log/slog"
	"path"

	"github.com/Hayao0819/Kamisato/internal/utils"
)

// NameStore is a cache keyed by (store arch, name); a miss falls through to the
// authoritative .db. storeArch is "any" for an arch=any package, else the
// concrete arch holding the file.
func (s *Service) resolvePackage(repo, reqArch, pkgname string) (filename, storeArch string, err error) {
	if reqArch != "" && reqArch != "any" {
		if f, e := s.pkgNameRepo.PackageFile(reqArch, pkgname); e == nil && f != "" {
			return f, reqArch, nil
		}
	}
	if f, e := s.pkgNameRepo.PackageFile("any", pkgname); e == nil && f != "" {
		return f, "any", nil
	}

	// Cache miss: read the .db, trying reqArch first then every configured arch
	// (an arch=any package is registered in each).
	var arches []string
	if reqArch != "" && reqArch != "any" {
		arches = append(arches, reqArch)
	}
	arches = append(arches, s.configuredArches(repo)...)
	for _, a := range arches {
		fn, sa, dbErr := s.packageEntryFromDB(repo, a, pkgname)
		if dbErr != nil {
			continue
		}
		// Backfill under the resolved store arch so later lookups hit.
		if storeErr := s.pkgNameRepo.StorePackageFile(sa, pkgname, fn); storeErr != nil {
			slog.Warn("failed to backfill name store", "arch", sa, "pkgname", pkgname, "error", storeErr.Error())
		}
		return fn, sa, nil
	}
	return "", "", fmt.Errorf("package %q not found in %s", pkgname, repo)
}

func (s *Service) packageEntryFromDB(repo, arch, pkgname string) (filename, pkgArch string, err error) {
	rr, err := s.pkgBinaryRepo.RemoteRepo(repo, arch)
	if err != nil {
		return "", "", utils.WrapErr(err, "fetch remote repo db")
	}

	p := rr.PkgByPkgName(pkgname)
	if p == nil {
		return "", "", fmt.Errorf("package %q not found in %s/%s db", pkgname, repo, arch)
	}

	// A .db package carries its file name in Path (%FILENAME%); take the base to
	// match what StorePackageFile holds.
	fn := path.Base(p.Path())
	if fn == "" || fn == "." || fn == "/" {
		return "", "", fmt.Errorf("package %q has no filename in %s/%s db", pkgname, repo, arch)
	}
	return fn, p.Arch(), nil
}
