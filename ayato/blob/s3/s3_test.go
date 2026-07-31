package s3

import (
	"os"
	"path"
	"testing"
)

// TestUploadFile is an opt-in integration test against a live S3/R2 endpoint; it
// reads settings from the environment and skips when no endpoint is configured.
func TestUploadFile(t *testing.T) {
	endpoint := os.Getenv("AYATO_STORE_AWSS3_ENDPOINT")
	if endpoint == "" {
		t.Skip("Skipping test because S3 endpoint is not set")
	}

	s3, err := New(&Config{
		Bucket:          os.Getenv("AYATO_STORE_AWSS3_BUCKET"),
		Region:          os.Getenv("AYATO_STORE_AWSS3_REGION"),
		Endpoint:        endpoint,
		AccessKeyID:     os.Getenv("AYATO_STORE_AWSS3_ACCESSKEYID"),
		SecretAccessKey: os.Getenv("AYATO_STORE_AWSS3_SECRETKEY"),
		SessionToken:    os.Getenv("AYATO_STORE_AWSS3_SESSIONTOKEN"),
		UsePathStyle:    os.Getenv("AYATO_STORE_AWSS3_USEPATHSTYLE") == "true",
	})
	if err != nil {
		t.Fatalf("Failed to create S3 client: %v", err)
	}

	tmpdir := t.TempDir()
	filePath := tmpdir + "/test.txt"
	file, err := os.Create(filePath)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	defer file.Close()
	defer func() { _ = os.Remove(filePath) }()
	_, err = file.WriteString("This is a test file.")
	if err != nil {
		t.Fatalf("Failed to write to test file: %v", err)
	}
	if _, err := file.Seek(0, 0); err != nil {
		t.Fatalf("Failed to rewind test file: %v", err)
	}

	err = s3.putObject(path.Base(filePath), file)
	if err != nil {
		t.Fatalf("Failed to upload file: %v", err)
	}
	t.Log("File uploaded successfully")
}
