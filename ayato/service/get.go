package service

import (
	"fmt"

	"github.com/Hayao0819/Kamisato/internal/errors"

	"github.com/Hayao0819/Kamisato/ayato/domain"
	"github.com/Hayao0819/Kamisato/ayato/repository/blob"
	"github.com/Hayao0819/Kamisato/ayato/stream"
)

// GetFileWithMeta serves a repository file with its conditional-GET metadata (ETag
// + last-modified). A blob miss maps to domain.ErrNotFound so the transport layer
// never inspects the blob package.
func (s *Service) GetFileWithMeta(repoName, archName, name string) (stream.File, domain.FileMeta, error) {
	f, meta, err := s.pkgBinaryRepo.FetchFileWithMeta(repoName, archName, name)
	if err != nil {
		if errors.Is(err, blob.ErrNotFound) {
			return nil, domain.FileMeta{}, fmt.Errorf("%w: %s", domain.ErrNotFound, name)
		}
		return nil, domain.FileMeta{}, err
	}
	return f, domain.FileMeta{ETag: meta.ETag, LastModified: meta.LastModified}, nil
}
