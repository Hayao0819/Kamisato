package service

import "github.com/Hayao0819/Kamisato/ayato/repository/provider"

func (s *Service) GetFile(repoName, archName, name string) (provider.BinaryStream, error) {
	pkg, err := s.r.FetchFile(repoName, archName, name)
	if err != nil {
		return nil, err
	}
	return pkg, nil
}
