package s3

import "github.com/Hayao0819/Kamisato/ayato/domain"

func (s *S3) prepareRepoExecuteEnv() (string, error) {
	return "", nil
}

func (s *S3) repoAdd(repo string, arch string, name string, pkgfile domain.IFileSeekStream, signfile domain.IFileSeekStream, useSignedDB bool, gnupgDir *string) error {
	_, err := s.prepareRepoExecuteEnv()
	return err
}

func (s *S3) repoRemove(repo string, arch string, name string, useSignedDB bool, gnupgDir *string) error {
	// return nil
	_, err := s.prepareRepoExecuteEnv()
	return err
}

func (s *S3) Init(repo string, arch string, useSignedDB bool, gnupgDir *string) error {
	// return nil
	_, err := s.prepareRepoExecuteEnv()

	if err != nil {
		return err
	}

	if err := s.repoAdd(repo, arch, "", nil, nil, useSignedDB, gnupgDir); err != nil {
		return err
	}

	return err
}
