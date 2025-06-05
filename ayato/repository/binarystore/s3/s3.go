package s3

import (
	"context"

	"github.com/Hayao0819/Kamisato/internal/conf"
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
