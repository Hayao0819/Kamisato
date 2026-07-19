package localfs

import (
	"os"
	"path"

	"github.com/Hayao0819/Kamisato/internal/errors"

	"github.com/Hayao0819/nahi/flist"
	"github.com/samber/lo"

	"github.com/Hayao0819/Kamisato/ayato/repository/blob"
)

func (l *LocalStore) RepoNames() ([]string, error) {
	if l.repoDir == "" {
		return nil, errors.New("local repository directory is not set")
	}
	dirs, err := flist.Get(
		l.repoDir,
		flist.WithDirOnly(),
		flist.WithExactDepth(1),
	)
	if err != nil {
		return nil, err
	}
	return lo.Map(*dirs, func(item string, _ int) string {
		return path.Base(item)
	}), nil
}

func (l *LocalStore) Arches(repo string) ([]string, error) {
	repoDir, err := l.getRepoDir(repo)
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(repoDir)
	if err != nil {
		return nil, err
	}
	arches := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			arches = append(arches, entry.Name())
		}
	}
	return arches, nil
}

func (l *LocalStore) Files(repo, arch string) ([]string, error) {
	repoPath, err := l.archPath(repo, arch)
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(repoPath)
	if errors.Is(err, os.ErrNotExist) {
		return []string{}, nil
	}
	if err != nil {
		return nil, err
	}
	files := make([]string, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			files = append(files, entry.Name())
		}
	}
	return files, nil
}

// FilesWithMeta retains entries whose stat/hash fails with empty metadata, so a
// transient filesystem error can never make orphan reconciliation delete them.
func (l *LocalStore) FilesWithMeta(
	repo, arch string,
) ([]blob.FileInfo, error) {
	repoPath, err := l.archPath(repo, arch)
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(repoPath)
	if errors.Is(err, os.ErrNotExist) {
		return []blob.FileInfo{}, nil
	}
	if err != nil {
		return nil, err
	}
	infos := make([]blob.FileInfo, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		objectPath := path.Join(repoPath, entry.Name())
		info := blob.FileInfo{Name: entry.Name()}
		if fileInfo, statErr := os.Stat(objectPath); statErr == nil {
			info.LastModified = fileInfo.ModTime()
			if version, versionErr := localFileETag(objectPath); versionErr == nil {
				info.Version = version
			}
		}
		infos = append(infos, info)
	}
	return infos, nil
}
