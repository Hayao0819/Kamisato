package localfs

import (
	"fmt"
	"os"
	"path"
)

func (l *LocalPkgBinaryStore) RepoNames() ([]string, error) {
	if l.cfg == nil {
		return nil, fmt.Errorf("config is nil")
	}
	names := make([]string, 0, len(l.cfg.RepoPath))
	for _, r := range l.cfg.RepoPath {
		names = append(names, path.Base(r))
	}
	return names, nil
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
