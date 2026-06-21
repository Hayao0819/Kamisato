package service

import (
	"github.com/Hayao0819/Kamisato/ayato/stream"
)

func (s *Service) GetFile(repoName, archName, name string) (stream.File, error) {
	pkg, err := s.pkgBinaryRepo.FetchFile(repoName, archName, name)
	if err != nil {
		return nil, err
	}
	return pkg, nil
}
