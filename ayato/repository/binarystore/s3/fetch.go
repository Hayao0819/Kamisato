package s3

import (
	"fmt"

	"github.com/Hayao0819/Kamisato/ayato/domain"
)

func (s *S3) FetchFile(repo string, arch string, name string) (domain.IFileStream, error) {
	o, err := s.getObject(key(repo, arch, name))
	if err != nil {
		return nil, err
	}
	if o == nil {
		return nil, fmt.Errorf("file %s/%s/%s not found", repo, arch, name)
	}
	return o, nil
}
