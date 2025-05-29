package s3

import (
	"fmt"
	"time"

	"github.com/Hayao0819/Kamisato/ayato/stream"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

func (s *S3) StoreFile(repo string, arch string, file stream.IFileSeekStream) error {
	k := key(repo, arch, file.FileName())
	if err := s.putObject(k, file); err != nil {
		return fmt.Errorf("failed to store file %s: %w", k, err)
	}

	if err := s.RepoAdd(repo, arch, file, nil, false, nil); err != nil {
		return fmt.Errorf("failed to add file %s to repo: %w", k, err)
	}
	return nil
}

func (s *S3) StoreFileWithSignedURL(repo string, arch string, name string) (string, error) {
	k := key(repo, arch, name)

	input := s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(k),
	}

	presignClient := s3.NewPresignClient(s.storage)
	presignResult, err := presignClient.PresignGetObject(s.ctx, &input, func(po *s3.PresignOptions) {
		po.Expires = 15 * time.Minute
	})

	if err != nil {
		return "", fmt.Errorf("failed to create presigned URL for %s: %w", k, err)
	}
	return presignResult.URL, nil
}
