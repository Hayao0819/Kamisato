package s3

import (
	"os"
	"path"
	"testing"
)

// TestUploadFile is an opt-in integration test against a live S3/R2 endpoint. It
// reads the connection settings straight from the environment so the IO layer
// (and its tests) stay free of the conf package; it skips when no endpoint is
// configured.
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

	// Create a file for the test
	tmpdir := t.TempDir()
	filePath := tmpdir + "/test.txt"
	file, err := os.Create(filePath)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	defer os.Remove(filePath) // Remove the file after the test
	_, err = file.WriteString("This is a test file.")
	if err != nil {
		t.Fatalf("Failed to write to test file: %v", err)
	}

	err = s3.putFile(path.Base(filePath), filePath)
	if err != nil {
		t.Fatalf("Failed to upload file: %v", err)
	}
	t.Log("File uploaded successfully")
}
