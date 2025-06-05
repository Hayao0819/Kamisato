package localfs

import (
	"fmt"
	"log/slog"
	"path"

	"github.com/Hayao0819/Kamisato/conf"
)

type LocalPkgBinaryStore struct {
	cfg *conf.AyatoConfig // Assume Config has RepoPath []string
}

func NewLocalPkgBinaryStore(cfg *conf.AyatoConfig) *LocalPkgBinaryStore {
	return &LocalPkgBinaryStore{cfg: cfg}
}

func (l *LocalPkgBinaryStore) getRepoDir(name string) (string, error) {
	if l.cfg == nil {
		return "", fmt.Errorf("config is nil")
	}
	slog.Debug("get repo dir", "name", name)
	for _, r := range l.cfg.Repos {
		if r.Name == name {
			return path.Join(l.cfg.Store.LocalRepoDir, name), nil
		}
	}
	return "", fmt.Errorf("repo %s not found", name)
}
