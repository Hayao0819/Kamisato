package s3

import (
	"os"
	"testing"

	"github.com/Hayao0819/Kamisato/conf"
)

func TestUploadFile(t *testing.T) {

	cfg, err := conf.LoadAyatoConfig(nil)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	t.Log(cfg.AWSS3.Endpoint)

	if cfg.AWSS3.Endpoint == "" {
		t.Skip("Skipping test because S3 endpoint is not set")
	}

	s3, err := NewS3(&cfg.AWSS3)
	if err != nil {
		t.Fatalf("Failed to create S3 client: %v", err)
	}

	// テスト用のファイルを作成
	tmpdir := t.TempDir()
	filePath := tmpdir + "/test.txt"
	file, err := os.Create(filePath)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	defer os.Remove(filePath) // テスト後にファイルを削除
	_, err = file.WriteString("This is a test file.")
	if err != nil {
		t.Fatalf("Failed to write to test file: %v", err)
	}

	err = s3.uploadFile(filePath)
	if err != nil {
		t.Fatalf("Failed to upload file: %v", err)
	}
	t.Log("File uploaded successfully")
}
