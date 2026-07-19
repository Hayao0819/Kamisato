package s3

import (
	"log/slog"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/samber/lo"
)

// listObjectsV2 consumes every continuation page. A delimiter limits results to
// one directory level; nil walks the full prefix subtree.
func (s *S3) listObjectsV2(
	prefix string,
	delimiter *string,
) (*awss3.ListObjectsV2Output, error) {
	merged := &awss3.ListObjectsV2Output{}
	var token *string
	for {
		page, err := s.storage.ListObjectsV2(s.ctx, &awss3.ListObjectsV2Input{
			Bucket:            aws.String(s.bucket),
			Prefix:            aws.String(prefix),
			Delimiter:         delimiter,
			ContinuationToken: token,
		})
		slog.Debug("S3 ListObjectsV2", "prefix", prefix, "bucket", s.bucket)
		if err != nil {
			return nil, err
		}
		merged.Contents = append(merged.Contents, page.Contents...)
		merged.CommonPrefixes = append(merged.CommonPrefixes, page.CommonPrefixes...)
		if !aws.ToBool(page.IsTruncated) || page.NextContinuationToken == nil {
			return merged, nil
		}
		token = page.NextContinuationToken
	}
}

func (s *S3) list(dir string) (*awss3.ListObjectsV2Output, error) {
	return s.listObjectsV2(dir, aws.String("/"))
}

func (s *S3) listDirs(dir string) ([]string, error) {
	result, err := s.list(dir)
	if err != nil {
		return nil, err
	}
	dirs := make([]string, 0, len(result.CommonPrefixes))
	for _, object := range result.CommonPrefixes {
		dirs = append(dirs, aws.ToString(object.Prefix))
	}
	return lo.Map(dirs, func(name string, _ int) string {
		return strings.TrimSuffix(name, "/")
	}), nil
}

func (s *S3) listFiles(dir string) ([]string, error) {
	result, err := s.list(dir)
	if err != nil {
		return nil, err
	}
	files := make([]string, 0, len(result.Contents))
	for _, object := range result.Contents {
		files = append(files, aws.ToString(object.Key))
	}
	return files, nil
}

func (s *S3) listAllKeys(prefix string) ([]string, error) {
	result, err := s.listObjectsV2(prefix, nil)
	if err != nil {
		return nil, err
	}
	keys := make([]string, 0, len(result.Contents))
	for _, object := range result.Contents {
		keys = append(keys, aws.ToString(object.Key))
	}
	return keys, nil
}
