package service

import (
	"errors"
	"fmt"

	"github.com/Hayao0819/Kamisato/ayato/domain"
	"github.com/Hayao0819/Kamisato/ayato/repository/blob"
	"github.com/Hayao0819/Kamisato/ayato/stream"
)

// GetFileWithMeta serves a repository file along with its conditional-GET metadata
// (ETag + last-modified), so the handler can answer If-None-Match and (for pacman)
// If-Modified-Since. The metadata is zero-valued for backends that expose none
// (a test store), which just means a full body. A blob miss is translated to
// domain.ErrNotFound so the transport layer never inspects the blob package.
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
