package s3

import (
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"
)

func (s *S3) list(dir string) (*awss3.ListObjectsV2Output, error) {
	l, err := s.storage.ListObjectsV2(s.ctx, &awss3.ListObjectsV2Input{
		Bucket:    aws.String(s.bucket),
		Prefix:    aws.String(dir),
		Delimiter: aws.String("/"),
	})
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
	return dirs, nil
}

func (s *S3) uploadFile(name string) error {

	f, err := os.Open(name)
	if err != nil {
		return err
	}
	_, err = s.storage.PutObject(s.ctx, &awss3.PutObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(name),
		Body:   f,
	})
	if err != nil {
		return err
	}

	return nil
}
