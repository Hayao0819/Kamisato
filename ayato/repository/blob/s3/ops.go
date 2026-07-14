package s3

import (
	"fmt"
	"io"
	"log/slog"
	"path"
	"time"

	"github.com/Hayao0819/Kamisato/internal/errors"

	"github.com/aws/aws-sdk-go-v2/aws"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/samber/lo"

	"github.com/Hayao0819/Kamisato/ayato/repository/blob"
	"github.com/Hayao0819/Kamisato/ayato/stream"
)

// StoreFile stores the package object only; the repo DB is updated separately by
// RepoAdd, so an arch=any file lands under "any/" without creating an "any" DB.
func (s *S3) StoreFile(repo string, arch string, file stream.SeekFile) error {
	k, err := s.validatedKey(repo, arch, file.FileName())
	if err != nil {
		return err
	}
	if err := s.putObject(k, file); err != nil {
		return fmt.Errorf("failed to store file %s: %w", k, err)
	}
	return nil
}

func (s *S3) StoreFileWithSignedURL(repo string, arch string, name string) (string, error) {
	k, err := s.validatedKey(repo, arch, name)
	if err != nil {
		return "", err
	}

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

// StoreFileWithSignedPutURL presigns a PUT to the final object key so a large
// package can be uploaded straight to R2, bypassing the server's request-body
// limit; the server finalizes the already-stored object afterwards.
func (s *S3) StoreFileWithSignedPutURL(repo string, arch string, name string) (string, error) {
	k, err := s.validatedKey(repo, arch, name)
	if err != nil {
		return "", err
	}

	input := awss3.PutObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(k),
	}

	presignClient := awss3.NewPresignClient(s.storage)
	presignResult, err := presignClient.PresignPutObject(s.ctx, &input, func(po *awss3.PresignOptions) {
		po.Expires = 15 * time.Minute
	})
	if err != nil {
		return "", fmt.Errorf("failed to create presigned PUT URL for %s: %w", k, err)
	}
	return presignResult.URL, nil
}

// FetchFile fetches an object by its exact name, with no pacman naming logic: the
// repo-DB artifacts are real objects written by the repository layer, so a bare
// <repo>.db is served directly.
func (s *S3) FetchFile(repo string, arch string, name string) (stream.File, error) {
	k, err := s.validatedKey(repo, arch, name)
	if err != nil {
		return nil, err
	}
	o, err := s.getObject(k)
	if err != nil {
		return nil, err
	}
	if o == nil {
		return nil, fmt.Errorf("file %s/%s/%s not found", repo, arch, name)
	}
	return o, nil
}

// FetchFileWithETag fetches an object together with its version token (ETag).
func (s *S3) FetchFileWithETag(repo, arch, name string) (stream.File, string, error) {
	k, err := s.validatedKey(repo, arch, name)
	if err != nil {
		return nil, "", err
	}
	return s.getObjectWithETag(k)
}

// FetchFileWithMeta fetches an object with its ETag and last-modified time, so the
// HTTP layer can answer both If-None-Match and (for pacman) If-Modified-Since.
func (s *S3) FetchFileWithMeta(repo, arch, name string) (stream.File, blob.FileMeta, error) {
	k, err := s.validatedKey(repo, arch, name)
	if err != nil {
		return nil, blob.FileMeta{}, err
	}
	return s.getObjectWithMeta(k)
}

// StoreFileIfMatch stores an object with compare-and-swap on its version, mapping
// a conflict to blob.ErrPreconditionFailed.
func (s *S3) StoreFileIfMatch(repo, arch string, file stream.SeekFile, etag string) error {
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return err
	}
	k, err := s.validatedKey(repo, arch, file.FileName())
	if err != nil {
		return err
	}
	if err := s.putObjectIfMatch(k, file, etag); err != nil {
		if errors.Is(err, blob.ErrPreconditionFailed) {
			return err
		}
		return fmt.Errorf("failed to store file %s: %w", k, err)
	}
	return nil
}

func (s *S3) DeleteFile(repo string, arch string, name string) error {
	k, err := s.validatedKey(repo, arch, name)
	if err != nil {
		return err
	}
	if err := s.deleteObject(k); err != nil {
		return fmt.Errorf("failed to delete file %s: %w", k, err)
	}
	return nil
}

func (s *S3) RepoNames() ([]string, error) {
	return s.listDirs("")
}

// validateListRepo applies the same repo guards as the object-key path: a single
// safe path element, gated by the allowlist when configured. The S3 keyspace is
// flat, so this is uniform validation rather than traversal defense.
func (s *S3) validateListRepo(repo string) error {
	if err := blob.ValidatePathComponent(repo); err != nil {
		return err
	}
	if len(s.repoNames) > 0 && !lo.Contains(s.repoNames, repo) {
		return fmt.Errorf("%w: repo %s", blob.ErrNotFound, repo)
	}
	return nil
}

func (s *S3) Arches(repo string) ([]string, error) {
	slog.Debug("get arches", "repo", repo)
	if err := s.validateListRepo(repo); err != nil {
		return nil, err
	}
	dl, err := s.listDirs(repo + "/")
	if err != nil {
		return nil, err
	}
	return lo.Map(dl, func(name string, _ int) string {
		return path.Base(name)
	}), nil
}

func (s *S3) Files(repo string, arch string) ([]string, error) {
	if err := s.validateListRepo(repo); err != nil {
		return nil, err
	}
	if err := blob.ValidatePathComponent(arch); err != nil {
		return nil, err
	}
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

// FilesWithMeta lists objects with their last-modified time, applying the same
// repo/arch validation as Files. ListObjectsV2 already returns LastModified, so
// this is Files plus the time the orphan reconcile ages objects by.
func (s *S3) FilesWithMeta(repo string, arch string) ([]blob.FileInfo, error) {
	if err := s.validateListRepo(repo); err != nil {
		return nil, err
	}
	if err := blob.ValidatePathComponent(arch); err != nil {
		return nil, err
	}
	l, err := s.list(fmt.Sprintf("%s/%s/", repo, arch))
	if err != nil {
		return nil, err
	}
	infos := make([]blob.FileInfo, 0, len(l.Contents))
	for _, obj := range l.Contents {
		infos = append(infos, blob.FileInfo{
			Name:         path.Base(aws.ToString(obj.Key)),
			LastModified: aws.ToTime(obj.LastModified),
		})
	}
	return infos, nil
}
