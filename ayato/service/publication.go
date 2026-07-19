package service

import "github.com/Hayao0819/Kamisato/internal/errors"

func (s *Service) acquirePublicationLease(repo string) (func(), error) {
	leaser, ok := s.pkgBinaryRepo.(interface {
		AcquirePublicationLease(string) (func(), error)
	})
	if !ok {
		return func() {}, nil
	}
	release, err := leaser.AcquirePublicationLease(repo)
	if err != nil {
		return nil, errors.WrapErr(err, "acquire repository publication lease")
	}
	return release, nil
}
