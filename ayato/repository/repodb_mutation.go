package repository

import (
	"fmt"
	"log/slog"
	"path"

	"github.com/Hayao0819/Kamisato/internal/errors"

	"github.com/Hayao0819/Kamisato/ayato/stream"
	pacmanrepo "github.com/Hayao0819/Kamisato/pkg/pacman/repo"
)

// RepoAdd is the one-item shorthand for RepoAddBatch.
func (r *binaryRepository) RepoAdd(
	repo, arch string,
	pkg, sig stream.SeekFile,
	useSignedDB bool,
	gnupgDir *string,
) error {
	return r.RepoAddBatch(repo, arch, []RepoAddItem{{Pkg: pkg, Sig: sig}}, useSignedDB, gnupgDir)
}

// RepoAddBatch publishes all items in one database read-modify-write. The
// canonical package set therefore never exposes a partial batch.
func (r *binaryRepository) RepoAddBatch(
	repo, arch string,
	items []RepoAddItem,
	useSignedDB bool,
	gnupgDir *string,
) error {
	defer r.dbMu.lock(repo + "/" + arch)()

	return withRepoDBTempDir("ayato-repodb-", func(dir string) error {
		pkgPaths, err := materializeRepoAddItems(dir, items)
		if err != nil {
			return err
		}
		return r.mutateDB(repo, arch, dir, useSignedDB, func(dbPath string) error {
			if err := validateAddExpectations(dbPath, items); err != nil {
				return err
			}
			if err := r.repoTool().RepoAddBatch(dbPath, pkgPaths, useSignedDB, gnupgDir); err != nil {
				slog.Error("repo db add batch", "err", err, "count", len(pkgPaths))
				return errors.WrapErr(err, "repo db add failed")
			}
			return nil
		})
	})
}

func materializeRepoAddItems(dir string, items []RepoAddItem) ([]string, error) {
	pkgPaths := make([]string, 0, len(items))
	for _, item := range items {
		pkgPath, err := writeSeekFileTo(dir, item.Pkg)
		if err != nil {
			return nil, err
		}
		if pkgPath == "" {
			continue
		}
		pkgPaths = append(pkgPaths, pkgPath)
		// Repository tools discover a detached package signature beside the
		// package under the conventional "<package>.sig" name.
		if err := writeSeekFileToPath(pkgPath+".sig", item.Sig); err != nil {
			return nil, err
		}
	}
	return pkgPaths, nil
}

// RepoRemove removes a package using the same optimistic transaction as add.
func (r *binaryRepository) RepoRemove(
	repo, arch, pkg string,
	useSignedDB bool,
	gnupgDir *string,
) error {
	return r.repoRemove(repo, arch, pkg, "", "", false, useSignedDB, gnupgDir)
}

func (r *binaryRepository) RepoRemoveIfMatch(
	repo, arch, pkg, expectedVersion, expectedFile string,
	useSignedDB bool,
	gnupgDir *string,
) error {
	return r.repoRemove(repo, arch, pkg, expectedVersion, expectedFile, true, useSignedDB, gnupgDir)
}

func (r *binaryRepository) repoRemove(
	repo, arch, pkg, expectedVersion, expectedFile string,
	conditional, useSignedDB bool,
	gnupgDir *string,
) error {
	defer r.dbMu.lock(repo + "/" + arch)()

	return withRepoDBTempDir("ayato-repodb-", func(dir string) error {
		return r.mutateDB(repo, arch, dir, useSignedDB, func(dbPath string) error {
			if !fileExists(dbPath) {
				if conditional {
					return ErrPackageChanged
				}
				return errors.New("repository database not found")
			}
			if conditional {
				alreadyRemoved, err := validateCurrentPackage(dbPath, pkg, expectedVersion, expectedFile)
				if err != nil || alreadyRemoved {
					return err
				}
			}
			if err := r.repoTool().RepoRemove(dbPath, pkg, useSignedDB, gnupgDir); err != nil {
				if errors.Is(err, pacmanrepo.ErrPackageNotFound) {
					return nil
				}
				slog.Error("repo db remove", "err", err)
				return errors.WrapErr(err, "repo db remove failed")
			}
			return nil
		})
	})
}

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
