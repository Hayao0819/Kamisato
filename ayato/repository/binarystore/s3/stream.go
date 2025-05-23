package s3

import (
	"io"
	"path"

	"github.com/Hayao0819/Kamisato/ayato/stream"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

func (s *S3) getObject(key string) (stream.IFileStream, error) {
	output, err := s.storage.GetObject(s.ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, err
	}

	return &s3ObjectStream{
		Body:        output.Body,
		filename:    path.Base(key),                   // またはS3タグ等から取得
		contentType: aws.ToString(output.ContentType), // Content-Typeがある場合
	}, nil
}

type s3ObjectStream struct {
	Body        io.ReadCloser
	filename    string
	contentType string
}

func (s *s3ObjectStream) Read(p []byte) (int, error) {
	return s.Body.Read(p)
}

func (s *s3ObjectStream) Close() error {
	return s.Body.Close()
}

func (s *s3ObjectStream) FileName() string {
	return s.filename
}

func (s *s3ObjectStream) ContentType() string {
	if s.contentType != "" {
		return s.contentType
	}
	return "application/octet-stream"
}
