package s3

import (
	"fmt"

	domain "github.com/Hayao0819/Kamisato/ayato/stream"
)

func (s *S3) StoreFile(repo string, arch string, file domain.IFileSeekStream) error {
	k := key(repo, arch, file.FileName())
	if err := s.putObject(k, file); err != nil {
		return fmt.Errorf("failed to store file %s: %w", k, err)
	}

	if err := s.RepoAdd(repo, arch, file, nil, false, nil); err != nil {
		return fmt.Errorf("failed to add file %s to repo: %w", k, err)
	}
	return nil
}
