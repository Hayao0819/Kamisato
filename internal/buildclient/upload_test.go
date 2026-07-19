package buildclient

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func TestUploadPackageFilesUsesValidatedMultipartRoute(t *testing.T) {
	directory := t.TempDir()
	packagePath := filepath.Join(directory, "bar-2-1-x86_64.pkg.tar.zst")
	signaturePath := packagePath + ".sig"
	writeFile(t, packagePath, "PKG2")
	writeFile(t, signaturePath, "SIG2")

	var gotPackage, gotSignature string
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/api/unstable/repos/myrepo/packages" {
			t.Errorf("unexpected path %q; presign/finalize must not be used", request.URL.Path)
			writer.WriteHeader(http.StatusBadRequest)
			return
		}
		if request.Header.Get("Authorization") != "Bearer tok" {
			t.Errorf("Authorization = %q", request.Header.Get("Authorization"))
		}
		if err := request.ParseMultipartForm(1 << 20); err != nil {
			t.Errorf("parse multipart: %v", err)
		}
		if headers := request.MultipartForm.File["package"]; len(headers) == 1 {
			file, _ := headers[0].Open()
			body, _ := io.ReadAll(file)
			_ = file.Close()
			gotPackage = headers[0].Filename + ":" + string(body)
		}
		if headers := request.MultipartForm.File["signature"]; len(headers) == 1 {
			file, _ := headers[0].Open()
			body, _ := io.ReadAll(file)
			_ = file.Close()
			gotSignature = headers[0].Filename + ":" + string(body)
		}
		writer.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	if err := UploadPackageFiles(context.Background(), server.URL, "tok", "myrepo", packagePath, signaturePath); err != nil {
		t.Fatal(err)
	}
	if want := filepath.Base(packagePath) + ":PKG2"; gotPackage != want {
		t.Fatalf("package part = %q, want %q", gotPackage, want)
	}
	if want := filepath.Base(signaturePath) + ":SIG2"; gotSignature != want {
		t.Fatalf("signature part = %q, want %q", gotSignature, want)
	}
}

func TestUploadPackageFilesNoPackages(t *testing.T) {
	if err := UploadPackageFiles(context.Background(), "http://example", "tok", "r", "only.sig"); err == nil {
		t.Fatal("expected an error when no package files are given")
	}
}
