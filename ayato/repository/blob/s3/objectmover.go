package s3

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"

	"github.com/Hayao0819/Kamisato/ayato/repository/blob"
)

var _ blob.ObjectMover = (*S3)(nil)

// CopyObject copies srcKey to dstKey server-side (R2/S3 CopyObject: no download).
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

// ListObjects returns every object key under prefix. Unlike the servable listing it
// omits the "/" delimiter, so it walks the whole subtree rather than one level.
func (s *S3) ListObjects(prefix string) ([]string, error) {
	var keys []string
	var token *string
	for {
		l, err := s.storage.ListObjectsV2(s.ctx, &awss3.ListObjectsV2Input{
			Bucket:            aws.String(s.bucket),
			Prefix:            aws.String(prefix),
			ContinuationToken: token,
		})
		if err != nil {
			return nil, fmt.Errorf("list objects %q: %w", prefix, err)
		}
		for _, o := range l.Contents {
			keys = append(keys, aws.ToString(o.Key))
		}
		if !aws.ToBool(l.IsTruncated) || l.NextContinuationToken == nil {
			break
		}
		token = l.NextContinuationToken
	}
	return keys, nil
}

// DeleteObject removes objKey; S3/R2 delete is idempotent (a missing key is not an error).
func (s *S3) DeleteObject(objKey string) error {
	if err := s.deleteObject(objKey); err != nil {
		return fmt.Errorf("delete object %s: %w", objKey, err)
	}
	return nil
}

// encodeCopySource builds the URL-encoded "bucket/key" CopySource, escaping each
// path segment while preserving the slashes that delimit them.
func encodeCopySource(bucket, key string) string {
	segs := strings.Split(key, "/")
	for i, seg := range segs {
		segs[i] = url.PathEscape(seg)
	}
	return bucket + "/" + strings.Join(segs, "/")
}
