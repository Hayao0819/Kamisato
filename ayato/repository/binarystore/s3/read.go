package s3

import (
	"fmt"
	"log/slog"
	"path"

	"github.com/samber/lo"
)

func (s *S3) RepoNames() ([]string, error) {
	l, err := s.listDirs("")
	if err != nil {
		return nil, err
	}
	return l, nil
}

func (s *S3) Arches(repo string) ([]string, error) {
	slog.Debug("get arches", "repo", repo)
	dl, err := s.listDirs(repo + "/")
	if err != nil {
		return nil, err
	}
	return lo.Map(dl, func(name string, _ int) string {
		return path.Base(name)
	}), nil
}

func (s *S3) Files(repo string, arch string) ([]string, error) {
	return s.listFiles(fmt.Sprintf("%s/%s", repo, arch))
}
