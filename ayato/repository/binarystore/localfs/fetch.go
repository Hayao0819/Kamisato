package localfs

import (
	"log/slog"
	"os"
	"path"

	"github.com/Hayao0819/Kamisato/ayato/domain"
	"github.com/Hayao0819/nahi/futils"
)

func (l *LocalPkgBinaryStore) FetchFile(repo string, arch string, file string) (domain.IFileStream, error) {
	repoDir, err := l.getRepoDir(repo)
	if err != nil {
		return nil, err
	}

	pkgPath := path.Join(repoDir, arch, file)

	if !futils.Exists(pkgPath) {
		return nil, os.ErrNotExist
	}
	slog.Info("fetch pkg file", "file", pkgPath)

	streamFile, err := openFileStreamWithTypeDetection(pkgPath)
	if err != nil {
		return nil, err
	}
	return streamFile, nil
}
