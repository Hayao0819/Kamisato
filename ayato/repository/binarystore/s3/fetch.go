package s3

import (
	"fmt"

		domain "github.com/Hayao0819/Kamisato/ayato/stream"
)

// func fileAliasResolver(repo, arch, filename string) string
func fileAliasResolver(repo, _, filename string) string {
	if filename == repo+".db" {
		return repo + ".db.tar.gz"
	}
	if filename == repo+".files" {
		return repo + ".files.tar.gz"
	}
	return filename
}

func (s *S3) FetchFile(repo string, arch string, name string) (domain.IFileStream, error) {
	if name == repo+".db" {
		name = repo + ".db.tar.gz"
	}
	if name == repo+".files" {
		name = repo + ".files.tar.gz"
	}

	o, err := s.getObject(key(repo, arch, name))
	if err != nil {
		return nil, err
	}
	if o == nil {
		return nil, fmt.Errorf("file %s/%s/%s not found", repo, arch, name)
	}
	return o, nil
}
