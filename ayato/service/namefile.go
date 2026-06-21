package service

import (
	"fmt"
	"log/slog"
	"path"

	"github.com/Hayao0819/Kamisato/internal/utils"
)

// resolvePackageFile maps a package name to its stored file name.
//
// The NameStore is a cache over the authoritative source, the alpm .db. On a
// miss we fall back to the .db: locate the package by pkgname, take its file
// name, backfill the NameStore, and return it. If the .db also lacks the
// package, the original not-found error from the NameStore is returned so
// callers keep their existing error semantics.
func (s *Service) resolvePackageFile(repo, arch, pkgname string) (string, error) {
	filename, err := s.pkgNameRepo.PackageFile(pkgname)
	// A non-empty filename with no error is a hit. Both a non-nil error and an
	// empty string count as a miss: NameStore backends disagree on how absence
	// surfaces (some error, cloudflarekv returns an empty value).
	if err == nil && filename != "" {
		return filename, nil
	}

	resolved, dbErr := s.packageFileFromDB(repo, arch, pkgname)
	if dbErr != nil {
		slog.Debug("name read-through miss", "pkgname", pkgname, "error", dbErr.Error())
		// Preserve the NameStore's not-found semantics for the caller.
		if err != nil {
			return "", err
		}
		return "", dbErr
	}

	if storeErr := s.pkgNameRepo.StorePackageFile(pkgname, resolved); storeErr != nil {
		slog.Warn("failed to backfill name store", "pkgname", pkgname, "error", storeErr.Error())
	}
	return resolved, nil
}

// packageFileFromDB resolves a package file name from the repository .db.
func (s *Service) packageFileFromDB(repo, arch, pkgname string) (string, error) {
	rr, err := s.pkgBinaryRepo.RemoteRepo(repo, arch)
	if err != nil {
		return "", utils.WrapErr(err, "fetch remote repo db")
	}

	p := rr.PkgByPkgName(pkgname)
	if p == nil {
		return "", fmt.Errorf("package %q not found in %s/%s db", pkgname, repo, arch)
	}

	// A package parsed from the .db carries its file name in Path (the desc
	// %FILENAME% field); take the base so it matches what StorePackageFile holds.
	filename := path.Base(p.Path())
	if filename == "" || filename == "." || filename == "/" {
		return "", fmt.Errorf("package %q has no filename in %s/%s db", pkgname, repo, arch)
	}
	return filename, nil
}
