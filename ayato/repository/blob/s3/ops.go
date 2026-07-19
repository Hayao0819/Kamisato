package s3

import (
	"fmt"
	"log/slog"
	"path"
	"time"

	"github.com/Hayao0819/Kamisato/internal/errors"

	"github.com/aws/aws-sdk-go-v2/aws"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"

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

	presignClient := awss3.NewPresignClient(s.storage)
	presignResult, err := presignClient.PresignGetObject(s.ctx, s.getObjectInput(k), func(po *awss3.PresignOptions) {
		po.Expires = 15 * time.Minute
	})
	if err != nil {
		return "", fmt.Errorf("failed to create presigned URL for %s: %w", k, err)
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
	if err := stream.Rewind(file); err != nil {
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

func (s *S3) Arches(repo string) ([]string, error) {
	slog.Debug("get arches", "repo", repo)
	if err := s.validateRepo(repo); err != nil {
		return nil, err
	}
	dl, err := s.listDirs(repo + "/")
	if err != nil {
		return nil, err
	}
	return baseNames(dl), nil
}

func (s *S3) Files(repo string, arch string) ([]string, error) {
	prefix, err := s.validatedArchPrefix(repo, arch)
	if err != nil {
		return nil, err
	}
	l, err := s.listFiles(prefix)
	if err != nil {
		return nil, err
	}

	files := baseNames(l)

	slog.Debug("get files", "repo", repo, "arch", arch, "files", files)
	return files, nil
}

func baseNames(names []string) []string {
	result := make([]string, len(names))
	for index, name := range names {
		result[index] = path.Base(name)
	}
	return result
}

// FilesWithMeta lists objects with their last-modified time, applying the same
// repo/arch validation as Files. ListObjectsV2 already returns LastModified, so
// this is Files plus the time the orphan reconcile ages objects by.
func (s *S3) FilesWithMeta(repo string, arch string) ([]blob.FileInfo, error) {
	prefix, err := s.validatedArchPrefix(repo, arch)
	if err != nil {
		return nil, err
	}
	l, err := s.list(prefix)
	if err != nil {
		return nil, err
	}
	infos := make([]blob.FileInfo, 0, len(l.Contents))
	for _, obj := range l.Contents {
		infos = append(infos, blob.FileInfo{
			Name:         path.Base(aws.ToString(obj.Key)),
			LastModified: aws.ToTime(obj.LastModified),
			Version:      aws.ToString(obj.ETag),
		})
	}
	return infos, nil
}

// ListObjects walks the complete prefix subtree (no delimiter).
func (s *S3) ListObjects(prefix string) ([]string, error) {
	keys, err := s.listAllKeys(prefix)
	if err != nil {
		return nil, fmt.Errorf("list objects %q: %w", prefix, err)
	}
	return keys, nil
}
