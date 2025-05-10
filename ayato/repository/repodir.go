package repository

import (
	"fmt"
	"log/slog"
	"os"
	"path"

	"github.com/Hayao0819/Kamisato/repo"
	"github.com/Hayao0819/nahi/futils"
)

func (r *Repository) PkgRepoDir(name string) (string, error) {
	if r.cfg == nil {
		return "", fmt.Errorf("config is nil")
	}
	for _, r := range r.cfg.RepoPath {
		if path.Base(r) == name {
			return r, nil
		}
	}
	return "", fmt.Errorf("repo %s not found", name)
}

func (r *Repository) PkgRepoAdd(name string, packageName string, useSignedDB bool, gnupgDir *string) error {
	repoDir, err := r.PkgRepoDir(name)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(path.Join(repoDir, "x86_64"), os.ModePerm); err != nil {
		return fmt.Errorf("mkdir %s err: %s", path.Join(repoDir, "x86_64"), err.Error())
	}

	repoDbPath := path.Join(repoDir, "x86_64", name+".db.tar.gz")
	err = repo.RepoAdd(repoDbPath, packageName, useSignedDB, gnupgDir)
	if err != nil {
		return fmt.Errorf("repo-add err: %s", err.Error())
	}

	return nil
}

func (r *Repository) PkgRepoRemove(name string, packageName string, useSignedDB bool, gnupgDir *string) error {
	repoDir, err := r.PkgRepoDir(name)
	if err != nil {
		return err
	}

	repoDbPath := path.Join(repoDir, "x86_64", name+".db.tar.gz")
	err = repo.RepoAdd(repoDbPath, packageName, useSignedDB, gnupgDir)
	if err != nil {
		return fmt.Errorf("repo-add err: %s", err.Error())
	}

	return nil
}

// func (r *Repository) initPkgRepoDir(name string, useSignedDB bool, gnupgDir *string) error {
// 	return r.PkgRepoAdd(name, "", useSignedDB, gnupgDir)
// }

func (r *Repository) PkgRepoNames() []string {
	if r.cfg == nil {
		return nil
	}
	names := make([]string, 0, len(r.cfg.RepoPath))
	for _, r := range r.cfg.RepoPath {
		names = append(names, path.Base(r))
	}
	return names
}

func (r *Repository) PkgRepoInit(useSignedDB bool, gnupgDir *string) error {
	for _, name := range r.PkgRepoNames() {
		slog.Info("init pkg repo", "name", name)
		if err := r.PkgRepoAdd(name, "", useSignedDB, gnupgDir); err != nil {
			return err
		}
	}
	return nil
}

func (r *Repository) PkgRepoFileList(name string, arch string) ([]string, error) {
	dp, err := r.PkgRepoDir(name)
	if err != nil {
		// slog.Error("err while getting repo dir", "name", name, "err", err)
		// continue
		return nil, err
	}

	entries, err := os.ReadDir(path.Join(dp, arch))
	if err != nil {
		return nil, err
	}

	rt := []string{}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		rt = append(rt, e.Name())
	}
	return rt, nil
}

func (r *Repository) VerifyPkgRepo(name string) error {
	dir, err := r.PkgRepoDir(name)
	if err != nil {
		return err
	}

	// if _, err := os.Stat(path.Join(dir, "x86_64")); os.IsNotExist(err) {
	// 	return fmt.Errorf("x86_64 directory not found in %s", dir)
	// }

	files := []string{
		name + ".db",
		name + ".db.tar.gz",
		name + ".files",
		name + ".files.tar.gz",
	}

	for _, file := range files {
		if futils.Exists(path.Join(dir, "x86_64", file)) {
			continue
		}
		return fmt.Errorf("%s not found in %s", file, path.Join(dir, "x86_64"))
	}
	return nil
}
