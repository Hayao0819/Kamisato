package service

import (
	"github.com/Hayao0819/Kamisato/ayato/domain"
	"github.com/Hayao0819/Kamisato/ayato/stream"
)

// GetFileWithMeta serves a repository file along with its conditional-GET metadata
// (ETag + last-modified), so the handler can answer If-None-Match and (for pacman)
// If-Modified-Since. The metadata is zero-valued for backends that expose none
// (a test store), which just means a full body.
func (s *Service) GetFileWithMeta(repoName, archName, name string) (stream.File, domain.FileMeta, error) {
	f, meta, err := s.pkgBinaryRepo.FetchFileWithMeta(repoName, archName, name)
	if err != nil {
		return nil, domain.FileMeta{}, err
	}
	return f, domain.FileMeta{ETag: meta.ETag, LastModified: meta.LastModified}, nil
}
