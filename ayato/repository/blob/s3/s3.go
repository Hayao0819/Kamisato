// Package s3 is a blob.Store backed by S3/R2-compatible object storage.
package s3

import (
	"context"
	"fmt"
	"net/http"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/retry"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/samber/lo"

	"github.com/Hayao0819/Kamisato/ayato/repository/blob"
	"github.com/Hayao0819/Kamisato/pkg/pacman/reponame"
)

var (
	_ blob.Store          = (*S3)(nil)
	_ blob.ObjectMover    = (*S3)(nil)
	_ blob.StagedUploader = (*S3)(nil)
)

// Config contains transport settings only, keeping this adapter independent of
// the application configuration package.
type Config struct {
	Bucket          string
	Region          string
	Endpoint        string
	AccessKeyID     string
	SecretAccessKey string
	SessionToken    string
	UsePathStyle    bool
	// RepoNames gates object access when non-empty, matching localfs.
	RepoNames []string
}

type S3 struct {
	storage   *awss3.Client
	ctx       context.Context
	bucket    string
	repoNames []string
}

func New(cfg *Config) (*S3, error) {
	if cfg == nil {
		return nil, fmt.Errorf("s3 config is required")
	}
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
	credentialsProvider := credentials.NewStaticCredentialsProvider(
		cfg.AccessKeyID,
		cfg.SecretAccessKey,
		cfg.SessionToken,
	)
	awsConfig, err := config.LoadDefaultConfig(
		ctx,
		config.WithRegion(cfg.Region),
		config.WithCredentialsProvider(credentialsProvider),
		config.WithRetryer(func() aws.Retryer {
			return retry.AddWithMaxAttempts(retry.NewStandard(), 3)
		}),
	)
	if err != nil {
		return nil, err
	}
	options := awss3.Options{
		Credentials:      awsConfig.Credentials,
		Region:           cfg.Region,
		UsePathStyle:     cfg.UsePathStyle,
		EndpointResolver: awss3.EndpointResolverFromURL(cfg.Endpoint),
		HTTPClient:       http.DefaultClient,
		Retryer:          awsConfig.Retryer(),
		// R2 rejects the SDK's default aws-chunked checksum trailer.
		RequestChecksumCalculation: aws.RequestChecksumCalculationWhenRequired,
		ResponseChecksumValidation: aws.ResponseChecksumValidationWhenRequired,
	}
	return awss3.New(options), nil
}

func key(repo, arch, name string) string {
	return repo + "/" + arch + "/" + name
}

func (s *S3) validateRepo(repo string) error {
	if err := reponame.Validate(repo); err != nil {
		return err
	}
	if len(s.repoNames) > 0 && !lo.Contains(s.repoNames, repo) {
		return fmt.Errorf("%w: repo %s", blob.ErrNotFound, repo)
	}
	return nil
}

func (s *S3) validatedKey(repo, arch, name string) (string, error) {
	if err := s.validateRepo(repo); err != nil {
		return "", err
	}
	if err := blob.ValidatePathComponent(arch); err != nil {
		return "", err
	}
	if err := blob.ValidatePathComponent(name); err != nil {
		return "", err
	}
	return key(repo, arch, name), nil
}

func (s *S3) validatedArchPrefix(repo, arch string) (string, error) {
	if err := s.validateRepo(repo); err != nil {
		return "", err
	}
	if err := blob.ValidatePathComponent(arch); err != nil {
		return "", err
	}
	return repo + "/" + arch + "/", nil
}
