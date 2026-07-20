package pacman

import (
	"errors"
	"fmt"

	alpm "github.com/Hayao0819/dyalpm"
)

type databases struct {
	handle alpm.Handle
	local  alpm.Database
	sync   []alpm.Database
}

func openDatabases(withSync bool) (*databases, error) {
	config, err := loadConfig("")
	if err != nil {
		return nil, err
	}

	handle, err := alpm.Initialize(config.RootDir, config.DBPath)
	if err != nil {
		return nil, fmt.Errorf("initialize alpm: %w", err)
	}
	fail := func(err error) (*databases, error) {
		return nil, errors.Join(err, handle.Release())
	}

	local, err := handle.LocalDB()
	if err != nil {
		return fail(fmt.Errorf("open local package database: %w", err))
	}
	result := &databases{handle: handle, local: local}
	if !withSync {
		return result, nil
	}

	result.sync = make([]alpm.Database, 0, len(config.Repos))
	for _, repo := range config.Repos {
		db, err := handle.RegisterSyncDB(repo.Name, 0)
		if err != nil {
			return fail(fmt.Errorf("register sync database %q: %w", repo.Name, err))
		}
		result.sync = append(result.sync, db)
	}
	return result, nil
}

func (dbs *databases) close(returnedErr *error) {
	*returnedErr = errors.Join(*returnedErr, dbs.handle.Release())
}

func InstalledVersions() (versions map[string]string, err error) {
	dbs, err := openDatabases(false)
	if err != nil {
		return nil, err
	}
	defer dbs.close(&err)

	versions = make(map[string]string)
	err = dbs.local.PkgCache().ForEach(func(pkg alpm.Package) error {
		versions[pkg.Name()] = pkg.Version()
		return nil
	})
	return versions, err
}

func Deptest(deps []string) (missing []string, err error) {
	if len(deps) == 0 {
		return nil, nil
	}
	dbs, err := openDatabases(false)
	if err != nil {
		return nil, err
	}
	defer dbs.close(&err)

	installed := dbs.local.PkgCache().Collect()
	for _, dep := range deps {
		if alpm.FindSatisfier(installed, dep) == nil {
			missing = append(missing, dep)
		}
	}
	return missing, nil
}

func ForeignPackages() (foreign map[string]bool, err error) {
	dbs, err := openDatabases(true)
	if err != nil {
		return nil, err
	}
	defer dbs.close(&err)

	syncNames, err := packageNames(dbs.sync)
	if err != nil {
		return nil, err
	}
	foreign = make(map[string]bool)
	err = dbs.local.PkgCache().ForEach(func(pkg alpm.Package) error {
		if !syncNames[pkg.Name()] {
			foreign[pkg.Name()] = true
		}
		return nil
	})
	return foreign, err
}

func SyncPackages() (packages map[string]bool, err error) {
	dbs, err := openDatabases(true)
	if err != nil {
		return nil, err
	}
	defer dbs.close(&err)
	return packageNames(dbs.sync)
}

func packageNames(databases []alpm.Database) (map[string]bool, error) {
	names := make(map[string]bool)
	for _, database := range databases {
		if validator, ok := database.(interface{ IsValid() bool }); ok && !validator.IsValid() {
			return nil, fmt.Errorf("sync database %q is invalid", database.Name())
		}
		if err := database.PkgCache().ForEach(func(pkg alpm.Package) error {
			names[pkg.Name()] = true
			return nil
		}); err != nil {
			return nil, fmt.Errorf("read sync database %q: %w", database.Name(), err)
		}
	}
	return names, nil
}
