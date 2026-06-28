// Package localfs is a binary store (blob.Store) backed by the local filesystem.
package localfs

import (
	"fmt"
	"log/slog"
	"os"
	"path"

	"github.com/Hayao0819/Kamisato/ayato/repository/blob"
	utils "github.com/Hayao0819/Kamisato/internal/utils"
	"github.com/samber/lo"
)

var _ blob.Store = (*LocalStore)(nil)

// LocalStore takes plain config values (repo root, known repo names) rather than
// the conf package, keeping the IO layer domain-free.
type LocalStore struct {
	repoDir   string
	repoNames []string
}

func New(repoDir string, repoNames []string) *LocalStore {
	return &LocalStore{repoDir: repoDir, repoNames: repoNames}
}

func (l *LocalStore) getRepoDir(name string) (string, error) {
	slog.Debug("get repo dir", "name", name)
	pwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to get current working directory: %w", err)
	}
	if lo.Contains(l.repoNames, name) {
		return path.Join(utils.ResolvePath(pwd, l.repoDir), name), nil
	}
	return "", fmt.Errorf("repo %s not found", name)
}
