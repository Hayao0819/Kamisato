package localfs

import (
	"fmt"
	"io"
	"os"
	"path"

	"github.com/Hayao0819/Kamisato/ayato/stream"
	"github.com/Hayao0819/Kamisato/internal/utils"
)

func (l *LocalPkgBinaryStore) StoreFile(repo string, arch string, file stream.IFileSeekStream) error {
	repoDir, err := l.getRepoDir(repo)
	if err != nil {
		return err
	}

	repoPath := path.Join(repoDir, arch)
	if err := os.MkdirAll(repoPath, os.ModePerm); err != nil {
		return utils.WrapErr(err, fmt.Sprintf("mkdir %s err", repoPath))
	}

	dstFilePath := path.Join(repoPath, file.FileName())
	// if err := cp.Copy(file, dstFile); err != nil {
	// 	return fmt.Errorf("copy file err: %s", err.Error())
	// }
	dstFile, err := os.Create(dstFilePath)
	if err != nil {
		return fmt.Errorf("create file err: %s", err.Error())
	}
	defer dstFile.Close()
	if _, err := io.Copy(dstFile, file); err != nil {
		return fmt.Errorf("copy file err: %s", err.Error())
	}

	// err = l.repoAdd(repo, arch, file.FileName(), useSignedDB, gnupgDir)
	// if err != nil {
	// 	return fmt.Errorf("repo-add err: %s", err.Error())
	// }
	return nil
}

func (l *LocalPkgBinaryStore) StoreFileWithSignedURL(repo string, arch string, name string) (string, error) {
	return "", nil
}
