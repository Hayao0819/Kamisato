package s3

func (s *S3) RepoNames() ([]string, error) {
	return s.listDirs("")
}

func (s *S3) Archeds(repo string) ([]string, error) {
	return s.listDirs(repo)
}
