package s3

import (
	"fmt"
	"log/slog"
	"path"
	"time"

	"github.com/Hayao0819/Kamisato/ayato/stream"
	"github.com/aws/aws-sdk-go-v2/aws"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/samber/lo"
)

// StoreFile stores the package object only. The database is updated by a
// separate RepoAdd call (the service drives both), matching the localfs store
// and letting an arch=any file be stored under "any/" without creating an "any"
// database.
func (s *S3) StoreFile(repo string, arch string, file stream.SeekFile) error {
	k := key(repo, arch, file.FileName())
	if err := s.putObject(k, file); err != nil {
		return fmt.Errorf("failed to store file %s: %w", k, err)
	}
	return nil
}

func (s *S3) StoreFileWithSignedURL(repo string, arch string, name string) (string, error) {
	k := key(repo, arch, name)

	input := awss3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(k),
	}

	presignClient := awss3.NewPresignClient(s.storage)
	presignResult, err := presignClient.PresignGetObject(s.ctx, &input, func(po *awss3.PresignOptions) {
		po.Expires = 15 * time.Minute
	})
	if err != nil {
		return "", fmt.Errorf("failed to create presigned URL for %s: %w", k, err)
	}
	return presignResult.URL, nil
}

// FetchFile fetches an object by its exact name — pure byte IO, with no pacman
// naming knowledge. The repo-DB artifacts (<repo>.db, .db.tar.gz, .files,
// .files.tar.gz) are each written as real objects by the repository layer's
// storeArtifacts, so the bare <repo>.db is served directly, the same as localfs.
func (s *S3) FetchFile(repo string, arch string, name string) (stream.File, error) {
	o, err := s.getObject(key(repo, arch, name))
	if err != nil {
		return nil, err
	}
	if o == nil {
		return nil, fmt.Errorf("file %s/%s/%s not found", repo, arch, name)
	}
	return o, nil
}

func (s *S3) DeleteFile(repo string, arch string, name string) error {
	k := key(repo, arch, name)
	if err := s.deleteObject(k); err != nil {
		return fmt.Errorf("failed to delete file %s: %w", k, err)
	}
	return nil
}

func (s *S3) RepoNames() ([]string, error) {
	return s.listDirs("")
}

func (s *S3) Arches(repo string) ([]string, error) {
	slog.Debug("get arches", "repo", repo)
	dl, err := s.listDirs(repo + "/")
	if err != nil {
		return nil, err
	}
	return lo.Map(dl, func(name string, _ int) string {
		return path.Base(name)
	}), nil
}

func (s *S3) Files(repo string, arch string) ([]string, error) {
	l, err := s.listFiles(fmt.Sprintf("%s/%s/", repo, arch))
	if err != nil {
		return nil, err
	}

	files := lo.Map(l, func(name string, _ int) string {
		return path.Base(name)
	})

	slog.Debug("get files", "repo", repo, "arch", arch, "files", files)
	return files, nil
}
