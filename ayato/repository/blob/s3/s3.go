// Package s3 is a binary store (blob.Store) backed by S3/R2-compatible storage.
package s3

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path"
	"strings"

	"github.com/Hayao0819/Kamisato/ayato/repository/blob"
	"github.com/Hayao0819/Kamisato/ayato/stream"
	"github.com/Hayao0819/Kamisato/internal/utils"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/retry"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go"
	smithyhttp "github.com/aws/smithy-go/transport/http"
	"github.com/samber/lo"
)

var _ blob.Store = (*S3)(nil)

// Config holds plain S3/R2 connection settings, decoupling the IO layer from the
// conf package.
type Config struct {
	Bucket          string
	Region          string
	Endpoint        string
	AccessKeyID     string
	SecretAccessKey string
	SessionToken    string
	UsePathStyle    bool
	// RepoNames is the configured repo allowlist; when set, keys for any other
	// repo are rejected (matching localfs). Empty means no allowlist gating.
	RepoNames []string
}

type S3 struct {
	storage   *awss3.Client
	ctx       context.Context
	bucket    string
	repoNames []string
}

func New(cfg *Config) (*S3, error) {
	ctx := context.Background()
	storage, err := newS3Client(ctx, cfg)
	if err != nil {
		return nil, err
	}
	return &S3{
		storage:   storage,
		ctx:       ctx,
		bucket:    cfg.Bucket,
		repoNames: cfg.RepoNames,
	}, nil
}

func newS3Client(ctx context.Context, cfg *Config) (*awss3.Client, error) {
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
		// R2 rejects the SDK's default flexible-checksum aws-chunked trailer;
		// compute a checksum only when an operation requires one. This also keeps
		// conditional (If-Match) PutObject working against R2.
		RequestChecksumCalculation: aws.RequestChecksumCalculationWhenRequired,
		ResponseChecksumValidation: aws.ResponseChecksumValidationWhenRequired,
	}

	return awss3.New(options), nil
}

func key(repo, arch, name string) string {
	return repo + "/" + arch + "/" + name
}

// validatePathComponent rejects values that could escape the key prefix.
func validatePathComponent(c string) error {
	if c == "" || c == "." || strings.ContainsRune(c, '/') || strings.ContainsRune(c, os.PathSeparator) || strings.Contains(c, "..") {
		return os.ErrNotExist
	}
	return nil
}

// validatedKey mirrors the localfs guards before building an object key: every
// component must be a single safe path element, and repo must be in the configured
// allowlist when one is set. Otherwise the raw repo/arch/name concatenation would
// let "../" or absolute components write outside the intended prefix.
func (s *S3) validatedKey(repo, arch, name string) (string, error) {
	if err := validatePathComponent(repo); err != nil {
		return "", err
	}
	if len(s.repoNames) > 0 && !lo.Contains(s.repoNames, repo) {
		return "", fmt.Errorf("repo %s not found", repo)
	}
	if err := validatePathComponent(arch); err != nil {
		return "", err
	}
	if err := validatePathComponent(name); err != nil {
		return "", err
	}
	return key(repo, arch, name), nil
}

func (s *S3) list(dir string) (*awss3.ListObjectsV2Output, error) {
	// ListObjectsV2 caps each call at 1000 keys; page through and merge.
	merged := &awss3.ListObjectsV2Output{}
	var token *string
	for {
		l, err := s.storage.ListObjectsV2(s.ctx, &awss3.ListObjectsV2Input{
			Bucket:            aws.String(s.bucket),
			Prefix:            aws.String(dir),
			Delimiter:         aws.String("/"),
			ContinuationToken: token,
		})
		slog.Debug("S3 ListObjectsV2", "dir", dir, "bucket", s.bucket, "result", l)
		if err != nil {
			return nil, err
		}

		merged.Contents = append(merged.Contents, l.Contents...)
		merged.CommonPrefixes = append(merged.CommonPrefixes, l.CommonPrefixes...)

		if !aws.ToBool(l.IsTruncated) || l.NextContinuationToken == nil {
			break
		}
		token = l.NextContinuationToken
	}
	return merged, nil
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
		if isNotFound(err) {
			return nil, blob.ErrNotFound
		}
		return nil, err
	}

	return &s3ObjectStream{
		Body:        output.Body,
		filename:    path.Base(key),
		contentType: aws.ToString(output.ContentType),
	}, nil
}

// isNotFound reports whether err is an object-absent error (NoSuchKey / 404), so
// callers can distinguish a true miss from a transient backend failure.
func isNotFound(err error) bool {
	var nsk *types.NoSuchKey
	if errors.As(err, &nsk) {
		return true
	}
	var nf *types.NotFound
	if errors.As(err, &nf) {
		return true
	}
	var re *smithyhttp.ResponseError
	return errors.As(err, &re) && re.HTTPStatusCode() == http.StatusNotFound
}

// getObjectWithETag fetches an object together with its ETag — the opaque version
// token for a compare-and-swap write.
func (s *S3) getObjectWithETag(key string) (stream.File, string, error) {
	f, meta, err := s.getObjectWithMeta(key)
	return f, meta.ETag, err
}

// getObjectWithMeta fetches an object together with its ETag and last-modified
// time, the two validators the HTTP layer needs for a conditional GET.
func (s *S3) getObjectWithMeta(key string) (stream.File, blob.FileMeta, error) {
	output, err := s.storage.GetObject(s.ctx, &awss3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		if isNotFound(err) {
			return nil, blob.FileMeta{}, blob.ErrNotFound
		}
		return nil, blob.FileMeta{}, err
	}
	return &s3ObjectStream{
		Body:        output.Body,
		filename:    path.Base(key),
		contentType: aws.ToString(output.ContentType),
	}, blob.FileMeta{ETag: aws.ToString(output.ETag), LastModified: aws.ToTime(output.LastModified)}, nil
}

// putObjectIfMatch writes with compare-and-swap: If-Match when etag is non-empty,
// else If-None-Match: * (create-only). The ETag is sent back verbatim (quotes
// included) as S3/R2 returned it. A precondition conflict (412 / 409) is mapped to
// blob.ErrPreconditionFailed so the caller can re-read and retry.
func (s *S3) putObjectIfMatch(key string, body io.ReadSeeker, etag string) error {
	in := &awss3.PutObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
		Body:   body,
	}
	if etag != "" {
		in.IfMatch = aws.String(etag)
	} else {
		in.IfNoneMatch = aws.String("*")
	}
	if _, err := s.storage.PutObject(s.ctx, in); err != nil {
		if isCASConflict(err) {
			return blob.ErrPreconditionFailed
		}
		return err
	}
	return nil
}

// isCASConflict reports a conditional-write conflict: 412 Precondition Failed (the
// ETag moved, or the object now exists) or 409 ConditionalRequestConflict (a
// racing in-flight writer). Both mean "re-read and retry".
func isCASConflict(err error) bool {
	var re *smithyhttp.ResponseError
	if errors.As(err, &re) {
		switch re.HTTPStatusCode() {
		case http.StatusPreconditionFailed, http.StatusConflict:
			return true
		}
	}
	var apiErr smithy.APIError
	if errors.As(err, &apiErr) {
		switch apiErr.ErrorCode() {
		case "PreconditionFailed", "ConditionalRequestConflict":
			return true
		}
	}
	return false
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
