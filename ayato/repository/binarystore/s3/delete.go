package s3

import "fmt"

func (s *S3) DeleteFile(repo string, arch string, name string) error {
	k := key(repo, arch, repo+".db.tar.gz")
	if err := s.deleteObject(k); err != nil {
		return fmt.Errorf("failed to delete file %s: %w", k, err)
	}

	// if err := s.RepoRemove(repo, arch, name, useSignedDB, gnupgDir); err != nil {
	// 	return fmt.Errorf("failed to remove file %s from repo: %w", k, err)
	// }
	return nil
}
