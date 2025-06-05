package localfs

import (
	"os"
	"path"

	"github.com/Hayao0819/nahi/flist"
)

func (l *LocalPkgBinaryStore) RepoNames() ([]string, error) {
	dirs, err := flist.Get(l.cfg.Store.LocalRepoDir, flist.WithDirOnly(), flist.WithExactDepth(1))
	if err != nil {
		return nil, err
	}
	return *dirs, nil
}

func (l *LocalPkgBinaryStore) FileList(name string, arch string) ([]string, error) {
	repoDir, err := l.getRepoDir(name)
	if err != nil {
		return nil, err
	}

	entries, err := os.ReadDir(path.Join(repoDir, arch))
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

func (l *LocalPkgBinaryStore) Arches(repo string) ([]string, error) {
	repoDir, err := l.getRepoDir(repo)
	if err != nil {
		return nil, err
	}

	entries, err := os.ReadDir(repoDir)
	if err != nil {
		return nil, err
	}

	archs := []string{}
	for _, entry := range entries {
		if entry.IsDir() {
			archs = append(archs, entry.Name())
		}
	}
	return archs, nil
}
