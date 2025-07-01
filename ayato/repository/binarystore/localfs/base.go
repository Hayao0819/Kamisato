package localfs

import (
	"fmt"
	"log/slog"
	"os"
	"path"

	"github.com/Hayao0819/Kamisato/internal/conf"
	utils "github.com/Hayao0819/Kamisato/internal/utils"
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
	pwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to get current working directory: %w", err)
	}
	for _, r := range l.cfg.Repos {
		if r.Name == name {
			return path.Join(utils.ResolvePath(pwd, l.cfg.Store.LocalRepoDir), name), nil
		}
	}
	return "", fmt.Errorf("repo %s not found", name)
}
