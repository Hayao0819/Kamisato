package service

import (
	"github.com/Hayao0819/Kamisato/ayato/repository/blob"
	"github.com/Hayao0819/Kamisato/ayato/stream"
)

// GetFileWithMeta serves a repository file along with its conditional-GET metadata
// (ETag + last-modified), so the handler can answer If-None-Match and (for pacman)
// If-Modified-Since. The metadata is zero-valued for backends that expose none
// (a test store), which just means a full body.
func (s *Service) GetFileWithMeta(repoName, archName, name string) (stream.File, blob.FileMeta, error) {
	return s.pkgBinaryRepo.FetchFileWithMeta(repoName, archName, name)
}
