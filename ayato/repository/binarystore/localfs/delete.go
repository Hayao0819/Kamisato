package localfs

import (
	"log/slog"
	"os"
	"path"

	"github.com/Hayao0819/Kamisato/internal/utils"
)

func (l *LocalPkgBinaryStore) DeleteFile(repo string, arch string, file string) error {
	repoDir, err := l.getRepoDir(repo)
	if err != nil {
		return err
	}

	// Remove package file to the repository directory
	pkgPath := path.Join(repoDir, "x86_64", file)
	slog.Info("remove pkg file", "file", pkgPath)
	if err := os.Remove(pkgPath); err != nil {
		slog.Warn("remove pkg file err", "err", err)
		return utils.WrapErr(err, "failed to remove pkg file")
	}

	return nil
}
