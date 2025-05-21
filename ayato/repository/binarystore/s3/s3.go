package s3

import (
	"context"
	"os"

	"github.com/Hayao0819/Kamisato/conf"
	"github.com/aws/aws-sdk-go-v2/aws"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"
)

type S3 struct {
	storage *awss3.Client
	ctx     context.Context
	bucket  string
}

func NewS3(cfg *conf.S3Config) (*S3, error) {
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
