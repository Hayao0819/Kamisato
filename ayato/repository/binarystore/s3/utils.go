package s3

import (
	"io"
	"log/slog"
	"os"
	"strings"

	"github.com/Hayao0819/Kamisato/internal/utils"
	"github.com/aws/aws-sdk-go-v2/aws"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/samber/lo"
)

func key(repo, arch, name string) string {
	return repo + "/" + arch + "/" + name
}

func (s *S3) list(dir string) (*awss3.ListObjectsV2Output, error) {
	l, err := s.storage.ListObjectsV2(s.ctx, &awss3.ListObjectsV2Input{
		Bucket:    aws.String(s.bucket),
		Prefix:    aws.String(dir),
		Delimiter: aws.String("/"),
	})
	slog.Debug("S3 ListObjectsV2", "dir", dir, "bucket", s.bucket, "result", l)
	if err != nil {
		return nil, err
	}
	return l, nil
}

func (s *S3) listDirs(dir string) ([]string, error) {
	l, err := s.list(dir)
	if err != nil {
		return nil, err
	}
	var dirs []string
	for _, obj := range l.CommonPrefixes {
		dirs = append(dirs, *obj.Prefix)
	}

	return lo.Map(dirs, func(name string, _ int) string {
		return strings.TrimSuffix(name, "/")
	}), nil
}

func (s *S3) listFiles(dir string) ([]string, error) {
	l, err := s.list(dir)
	if err != nil {
		return nil, err
	}
	// slog.Debug("get raw files", "dir", dir, "files", l.Contents)
	var files []string
	for _, obj := range l.Contents {
		files = append(files, *obj.Key)
	}
	return files, nil
}

func (s *S3) putFile(key, name string) error {

	f, err := os.Open(name)
	if err != nil {
		return err
	}
	defer f.Close()
	if err := s.putObject(key, f); err != nil {
		return utils.WrapErr(err, "failed to put object")
	}

	return nil
}

func (s *S3) putObject(key string, body io.ReadCloser) error {
	_, err := s.storage.PutObject(s.ctx, &awss3.PutObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
		Body:   body,
	})
	if err != nil {
		return err
	}
	return nil
}

func (s *S3) deleteObject(key string) error {
	_, err := s.storage.DeleteObject(s.ctx, &awss3.DeleteObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return err
	}
	return nil
}
