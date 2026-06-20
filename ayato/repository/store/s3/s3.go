// Package s3 は S3/R2 互換ストレージ上のバイナリストア(repository.Store)です。
package s3

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path"
	"strings"

	"github.com/Hayao0819/Kamisato/ayato/stream"
	"github.com/Hayao0819/Kamisato/internal/conf"
	"github.com/Hayao0819/Kamisato/internal/utils"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/retry"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/samber/lo"
)

type S3 struct {
	storage *awss3.Client
	ctx     context.Context
	bucket  string
}

func New(cfg *conf.S3Config) (*S3, error) {
	ctx := context.Background()
	storage, err := newS3Client(ctx, cfg)
	if err != nil {
		return nil, err
	}
	return &S3{
		storage: storage,
		ctx:     ctx,
		bucket:  cfg.Bucket,
	}, nil
}

func newS3Client(ctx context.Context, cfg *conf.S3Config) (*awss3.Client, error) {
	creds := credentials.NewStaticCredentialsProvider(
		cfg.AccessKeyID,
		cfg.SecretAccessKey,
		cfg.SessionToken,
	)

	awsCfg, err := config.LoadDefaultConfig(ctx,
		config.WithRegion(cfg.Region),
		config.WithCredentialsProvider(creds),
		config.WithRetryer(func() aws.Retryer {
			return retry.AddWithMaxAttempts(retry.NewStandard(), 3)
		}),
	)
	if err != nil {
		return nil, err
	}

	options := awss3.Options{
		Credentials:      awsCfg.Credentials,
		Region:           cfg.Region,
		UsePathStyle:     cfg.UsePathStyle,
		EndpointResolver: awss3.EndpointResolverFromURL(cfg.Endpoint),
		HTTPClient:       http.DefaultClient,
		Retryer:          awsCfg.Retryer(),
	}

	return awss3.New(options), nil
}

// --- low-level object operations ---

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
	return err
}

func (s *S3) deleteObject(key string) error {
	_, err := s.storage.DeleteObject(s.ctx, &awss3.DeleteObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	return err
}

func (s *S3) getObject(key string) (stream.File, error) {
	output, err := s.storage.GetObject(s.ctx, &awss3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, err
	}

	return &s3ObjectStream{
		Body:        output.Body,
		filename:    path.Base(key),
		contentType: aws.ToString(output.ContentType),
	}, nil
}

type s3ObjectStream struct {
	Body        io.ReadCloser
	filename    string
	contentType string
}

func (s *s3ObjectStream) Read(p []byte) (int, error) { return s.Body.Read(p) }
func (s *s3ObjectStream) Close() error               { return s.Body.Close() }
func (s *s3ObjectStream) FileName() string           { return s.filename }
func (s *s3ObjectStream) ContentType() string {
	if s.contentType != "" {
		return s.contentType
	}
	return "application/octet-stream"
}

// --- temp file helpers ---

func writeReadSeekerToFile(name string, stream io.Reader) error {
	file, err := os.Create(name)
	if err != nil {
		return err
	}
	defer file.Close()

	if seeker, ok := stream.(io.ReadSeeker); ok {
		if _, err = seeker.Seek(0, 0); err != nil {
			return err
		}
	}
	if _, err := io.Copy(file, stream); err != nil {
		return err
	}
	if seeker, ok := stream.(io.ReadSeeker); ok {
		if _, err := seeker.Seek(0, 0); err != nil {
			return utils.WrapErr(err, "failed to seek stream after writing to file")
		}
	}
	return nil
}

func writeStreamToFile(dir string, stream stream.File) (string, error) {
	if stream == nil {
		return "", nil
	}
	fp := path.Join(dir, stream.FileName())
	if err := writeReadSeekerToFile(fp, stream); err != nil {
		return "", err
	}

	return fp, nil
}
