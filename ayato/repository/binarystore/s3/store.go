package s3

import (
	"fmt"

	"github.com/Hayao0819/Kamisato/ayato/domain"
)

func (s *S3) StoreFile(repo string, arch string, file domain.IFileSeekStream, useSignedDB bool, gnupgDir *string) error {
	k := key(repo, arch, file.FileName())
	if err := s.putObject(k, file); err != nil {
		return fmt.Errorf("failed to store file %s: %w", k, err)
	}

	if err := s.repoAdd(repo, arch, file, false, nil); err != nil {
		return fmt.Errorf("failed to add file %s to repo: %w", k, err)
	}
	return nil
}
