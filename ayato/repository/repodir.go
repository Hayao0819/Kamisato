package repository

import (
	"fmt"
	"path"

	"github.com/Hayao0819/Kamisato/repo"
)

func (r *Repository) RepoDir(name string) (string, error) {
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

func (r *Repository) initRepoDir(name string, useSignedDB bool, gnupgDir *string) error {
	repoDir, err := r.RepoDir(name)
	if err != nil {
		return err
	}

	if err := repo.RepoInit(repoDir, useSignedDB, gnupgDir); err != nil {
		return fmt.Errorf("repo init err: %s", err.Error())
	}

	return nil
}

func (r *Repository) RepoNames() []string {
	if r.cfg == nil {
		return nil
	}
	names := make([]string, 0, len(r.cfg.RepoPath))
	for _, r := range r.cfg.RepoPath {
		names = append(names, path.Base(r))
	}
	return names
}

func (r *Repository) InitPacmanRepo(useSignedDB bool, gnupgDir *string) error {
	for _, name := range r.RepoNames() {
		if err := r.initRepoDir(name, useSignedDB, gnupgDir); err != nil {
			return err
		}
	}
	return nil
}
