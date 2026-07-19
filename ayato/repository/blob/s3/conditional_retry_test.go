package s3

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"sync/atomic"
	"testing"

	awsretry "github.com/aws/aws-sdk-go-v2/aws/retry"
	"github.com/aws/aws-sdk-go-v2/credentials"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"

	"github.com/Hayao0819/Kamisato/ayato/repository/blob"
)

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestConditionalPutDoesNotRetryAmbiguousTransportFailure(t *testing.T) {
	var attempts atomic.Int32
	client := awss3.New(awss3.Options{
		Credentials:  credentials.NewStaticCredentialsProvider("access", "secret", ""),
		Region:       "auto",
		UsePathStyle: true,
		EndpointResolver: awss3.EndpointResolverFromURL(
			"https://s3.invalid",
		),
		HTTPClient: &http.Client{Transport: roundTripperFunc(func(*http.Request) (*http.Response, error) {
			attempts.Add(1)
			return nil, io.ErrUnexpectedEOF
		})},
		Retryer: awsretry.AddWithMaxAttempts(awsretry.NewStandard(), 3),
	})
	s := &S3{storage: client, ctx: context.Background(), bucket: "bucket"}

	err := s.putObjectIfMatch("repo/x86_64/pkg", bytes.NewReader([]byte("payload")), `"old-etag"`)
	if err == nil {
		t.Fatal("conditional PUT unexpectedly succeeded")
	}
	if errors.Is(err, blob.ErrPreconditionFailed) {
		t.Fatalf("ambiguous transport failure was collapsed into a precondition conflict: %v", err)
	}
	if got := attempts.Load(); got != 1 {
		t.Fatalf("conditional PUT attempts = %d, want exactly 1", got)
	}
}
