// Package localfs はローカルファイルシステム上のバイナリストア(repository.Store)です。
package localfs

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path"

	"github.com/Hayao0819/Kamisato/ayato/stream"
	"github.com/Hayao0819/Kamisato/internal/conf"
	utils "github.com/Hayao0819/Kamisato/internal/utils"
)

type LocalStore struct {
	cfg *conf.AyatoConfig
}

func New(cfg *conf.AyatoConfig) *LocalStore {
	return &LocalStore{cfg: cfg}
}

func (l *LocalStore) getRepoDir(name string) (string, error) {
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

func writeReadSeekerToFile(name string, stream io.Reader) error {
	file, err := os.Create(name)
	if err != nil {
		return utils.WrapErr(err, "failed to create file")
	}
	defer file.Close()

	if seeker, ok := stream.(io.ReadSeeker); ok {
		if _, err = seeker.Seek(0, 0); err != nil {
			return utils.WrapErr(err, "failed to seek stream")
		}
	}
	if _, err := io.Copy(file, stream); err != nil {
		return utils.WrapErr(err, "failed to copy stream to file")
	}
	if seeker, ok := stream.(io.ReadSeeker); ok {
		if _, err := seeker.Seek(0, 0); err != nil {
			return utils.WrapErr(err, "failed to seek stream after writing to file")
		}
	}
	return nil
}

func writeStreamToFile(dir string, stream stream.File) (string, error) {
	if stream == nil {
		return "", nil
	}
	fp := path.Join(dir, stream.FileName())
	if err := writeReadSeekerToFile(fp, stream); err != nil {
		return "", err
	}

	return fp, nil
}
