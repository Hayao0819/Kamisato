package s3

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"

	"github.com/Hayao0819/Kamisato/internal/errors"

	"github.com/aws/aws-sdk-go-v2/aws"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go"
	smithyhttp "github.com/aws/smithy-go/transport/http"

	"github.com/Hayao0819/Kamisato/ayato/repository/blob"
	"github.com/Hayao0819/Kamisato/ayato/stream"
)

func (s *S3) putObject(objectKey string, body io.ReadCloser) error {
	_, err := s.storage.PutObject(s.ctx, &awss3.PutObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(objectKey),
		Body:   body,
	})
	return err
}

func (s *S3) deleteObject(objectKey string) error {
	_, err := s.storage.DeleteObject(s.ctx, &awss3.DeleteObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(objectKey),
	})
	return err
}

func (s *S3) getObject(objectKey string) (stream.File, error) {
	file, _, err := s.getObjectWithMeta(objectKey)
	return file, err
}

func (s *S3) getObjectWithETag(
	objectKey string,
) (stream.File, string, error) {
	file, meta, err := s.getObjectWithMeta(objectKey)
	return file, meta.ETag, err
}

func (s *S3) getObjectWithMeta(
	objectKey string,
) (stream.File, blob.FileMeta, error) {
	output, err := s.storage.GetObject(s.ctx, &awss3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(objectKey),
	})
	if err != nil {
		if isNotFound(err) {
			return nil, blob.FileMeta{}, blob.ErrNotFound
		}
		return nil, blob.FileMeta{}, err
	}
	file := &s3ObjectStream{
		Body:        output.Body,
		filename:    path.Base(objectKey),
		contentType: aws.ToString(output.ContentType),
	}
	meta := blob.FileMeta{
		ETag:         aws.ToString(output.ETag),
		LastModified: aws.ToTime(output.LastModified),
	}
	return file, meta, nil
}

func isNotFound(err error) bool {
	var noSuchKey *types.NoSuchKey
	if errors.As(err, &noSuchKey) {
		return true
	}
	var notFound *types.NotFound
	if errors.As(err, &notFound) {
		return true
	}
	var responseErr *smithyhttp.ResponseError
	return errors.As(err, &responseErr) &&
		responseErr.HTTPStatusCode() == http.StatusNotFound
}

// putObjectIfMatch performs a non-retried conditional mutation. Retrying an
// ambiguous transport failure could turn a successful create into a false
// precondition conflict.
func (s *S3) putObjectIfMatch(
	objectKey string,
	body io.ReadSeeker,
	etag string,
) error {
	input := &awss3.PutObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(objectKey),
		Body:   body,
	}
	if etag != "" {
		input.IfMatch = aws.String(etag)
	} else {
		input.IfNoneMatch = aws.String("*")
	}
	_, err := s.storage.PutObject(s.ctx, input, func(options *awss3.Options) {
		options.Retryer = aws.NopRetryer{}
	})
	if err != nil && isCASConflict(err) {
		return blob.ErrPreconditionFailed
	}
	return err
}

func isCASConflict(err error) bool {
	var responseErr *smithyhttp.ResponseError
	if errors.As(err, &responseErr) {
		switch responseErr.HTTPStatusCode() {
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

func (s *S3) CopyObject(srcKey, dstKey string) error {
	_, err := s.storage.CopyObject(s.ctx, &awss3.CopyObjectInput{
		Bucket:     aws.String(s.bucket),
		Key:        aws.String(dstKey),
		CopySource: aws.String(encodeCopySource(s.bucket, srcKey)),
	})
	if err != nil {
		return fmt.Errorf("copy object %s -> %s: %w", srcKey, dstKey, err)
	}
	return nil
}

func (s *S3) DeleteObject(objectKey string) error {
	if err := s.deleteObject(objectKey); err != nil {
		return fmt.Errorf("delete object %s: %w", objectKey, err)
	}
	return nil
}

func encodeCopySource(bucket, objectKey string) string {
	segments := strings.Split(objectKey, "/")
	for index, segment := range segments {
		segments[index] = url.PathEscape(segment)
	}
	return bucket + "/" + strings.Join(segments, "/")
}

type s3ObjectStream struct {
	Body        io.ReadCloser
	filename    string
	contentType string
}

func (s *s3ObjectStream) Read(data []byte) (int, error) { return s.Body.Read(data) }
func (s *s3ObjectStream) Close() error                  { return s.Body.Close() }
func (s *s3ObjectStream) FileName() string              { return s.filename }
func (s *s3ObjectStream) ContentType() string {
	if s.contentType != "" {
		return s.contentType
	}
	return "application/octet-stream"
}
