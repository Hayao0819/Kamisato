package repository

import (
	"fmt"
	"path"

	"github.com/Hayao0819/Kamisato/internal/errors"
	pacmanrepo "github.com/Hayao0819/Kamisato/pkg/pacman/repo"
)

func validateAddExpectations(dbPath string, items []RepoAddItem) error {
	if !hasConditionalAdd(items) {
		return nil
	}
	current, err := readRepoOrEmpty(dbPath)
	if err != nil {
		return err
	}
	for _, item := range items {
		if item.CheckCurrent {
			if err := validateAddExpectation(current, item); err != nil {
				return err
			}
		}
	}
	return nil
}

func hasConditionalAdd(items []RepoAddItem) bool {
	for _, item := range items {
		if item.CheckCurrent {
			return true
		}
	}
	return false
}

func readRepoOrEmpty(dbPath string) (*pacmanrepo.RemoteRepo, error) {
	if !fileExists(dbPath) {
		return &pacmanrepo.RemoteRepo{}, nil
	}
	current, err := pacmanrepo.RepoFromDBFile("", dbPath)
	if err != nil {
		return nil, errors.WrapErr(err, "read repo db for conditional publish")
	}
	return current, nil
}

func validateAddExpectation(current *pacmanrepo.RemoteRepo, item RepoAddItem) error {
	pkg := current.PkgByPkgName(item.ExpectedName)
	if pkg != nil &&
		item.IntendedVersion != "" &&
		pkg.Version() == item.IntendedVersion &&
		path.Base(pkg.Path()) == item.IntendedFile {
		return nil
	}
	if item.ExpectedCurrentVersion == "" {
		if pkg != nil {
			return fmt.Errorf("%w: %s was added", ErrPackageChanged, item.ExpectedName)
		}
		return nil
	}
	if pkg == nil ||
		pkg.Version() != item.ExpectedCurrentVersion ||
		path.Base(pkg.Path()) != item.ExpectedCurrentFile {
		return fmt.Errorf(
			"%w: %s no longer matches %s/%s",
			ErrPackageChanged,
			item.ExpectedName,
			item.ExpectedCurrentVersion,
			item.ExpectedCurrentFile,
		)
	}
	return nil
}

func validateCurrentPackage(
	dbPath, name, expectedVersion, expectedFile string,
) (alreadyRemoved bool, err error) {
	current, err := pacmanrepo.RepoFromDBFile("", dbPath)
	if err != nil {
		return false, errors.WrapErr(err, "read repo db for conditional remove")
	}
	pkg := current.PkgByPkgName(name)
	if pkg == nil {
		return true, nil
	}
	if pkg.Version() != expectedVersion || path.Base(pkg.Path()) != expectedFile {
		return false, fmt.Errorf(
			"%w: %s no longer matches %s/%s",
			ErrPackageChanged,
			name,
			expectedVersion,
			expectedFile,
		)
	}
	return false, nil
}
