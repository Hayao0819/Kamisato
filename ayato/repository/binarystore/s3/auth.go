package s3

import (
	"context"
	"net/http"

	"github.com/Hayao0819/Kamisato/internal/conf"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/retry"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

func newS3Client(ctx context.Context, cfg *conf.S3Config) (*s3.Client, error) {
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

	// エンドポイントのカスタマイズ
	options := s3.Options{
		Credentials:      awsCfg.Credentials,
		Region:           cfg.Region,
		UsePathStyle:     cfg.UsePathStyle,
		EndpointResolver: s3.EndpointResolverFromURL(cfg.Endpoint),
		HTTPClient:       http.DefaultClient,
		Retryer:          awsCfg.Retryer(),
	}

	client := s3.New(options)
	return client, nil
}
