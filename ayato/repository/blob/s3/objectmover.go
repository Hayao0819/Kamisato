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

// ListObjects omits the "/" delimiter, so it walks the whole subtree, not one level.
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

func (s *S3) DeleteObject(objKey string) error {
	if err := s.deleteObject(objKey); err != nil {
		return fmt.Errorf("delete object %s: %w", objKey, err)
	}
	return nil
}

// encodeCopySource URL-encodes each segment of the "bucket/key" CopySource while
// keeping the delimiting slashes, which the S3 API requires.
func encodeCopySource(bucket, key string) string {
	segs := strings.Split(key, "/")
	for i, seg := range segs {
		segs[i] = url.PathEscape(seg)
	}
	return bucket + "/" + strings.Join(segs, "/")
}
