package localfs

import (
	"os"
	"path"
)

func (l *LocalPkgBinaryStore) Files(repo string, arch string) ([]string, error) {
	repoDir, err := l.getRepoDir("ayato")
	if err != nil {
		return nil, err
	}
	repoPath := path.Join(repoDir, arch)

	entries, err := os.ReadDir(repoPath)
	if err != nil {
		return nil, err
	}
	files := []string{}
	for _, entry := range entries {
		if !entry.IsDir() {
			files = append(files, entry.Name())
		}
	}
	return files, nil
}
