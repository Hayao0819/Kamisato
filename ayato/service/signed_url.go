package service

import (
	"net/url"
)

// SignedURL は署名付きURLを生成します。
func (s *Service) SignedURL(repo, arch, name string) (string, error) {
	u, err := s.pkgBinaryRepo.StoreFileWithSignedURL(repo, arch, name)
	if err != nil {
		return "", err
	} else if u == "" {
		return "", nil
	}
	if _, err = url.Parse(u); err != nil {
		return "", err
	}
	return u, nil
}
