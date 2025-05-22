package s3

import "fmt"

func (s *S3) RepoNames() ([]string, error) {
	return s.listDirs("")
}

func (s *S3) Arches(repo string) ([]string, error) {
	return s.listDirs(repo)
}

func (s *S3) Files(repo string, arch string) ([]string, error) {
	return s.listFiles(fmt.Sprintf("%s/%s", repo, arch))
}
