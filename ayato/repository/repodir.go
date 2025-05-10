package repository

import (
	"fmt"
	"path"
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
