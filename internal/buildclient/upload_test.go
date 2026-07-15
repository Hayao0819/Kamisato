package buildclient

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func TestUploadPackageFilesPresign(t *testing.T) {
	dir := t.TempDir()
	pkg := filepath.Join(dir, "foo-1-1-x86_64.pkg.tar.zst")
	sig := pkg + ".sig"
	writeFile(t, pkg, "PKGBYTES")
	writeFile(t, sig, "SIGBYTES")

	stored := map[string]string{}
	storage := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("storage method = %q", r.Method)
		}
		if r.Header.Get("Authorization") != "" {
			t.Errorf("PUT carried an Authorization header: %q", r.Header.Get("Authorization"))
		}
		body, _ := io.ReadAll(r.Body)
		stored[strings.TrimPrefix(r.URL.Path, "/put/")] = string(body)
		w.WriteHeader(http.StatusOK)
	}))
	defer storage.Close()

	var presignReq struct {
		Files []string `json:"files"`
	}
	var finalizeReq struct {
		Packages []string `json:"packages"`
	}
	finalized := false
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer tok" {
			t.Errorf("api auth = %q", r.Header.Get("Authorization"))
		}
		switch {
		case strings.HasSuffix(r.URL.Path, "/packages/presign"):
			_ = json.NewDecoder(r.Body).Decode(&presignReq)
			urls := map[string]string{}
			for _, f := range presignReq.Files {
				urls[f] = storage.URL + "/put/" + f
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"urls": urls})
		case strings.HasSuffix(r.URL.Path, "/packages/finalize"):
			_ = json.NewDecoder(r.Body).Decode(&finalizeReq)
			finalized = true
			w.WriteHeader(http.StatusOK)
		default:
			t.Errorf("unexpected api path %q", r.URL.Path)
			w.WriteHeader(http.StatusBadRequest)
		}
	}))
	defer api.Close()

	if err := UploadPackageFiles(context.Background(), api.URL, "tok", "myrepo", pkg, sig); err != nil {
		t.Fatalf("UploadPackageFiles: %v", err)
	}

	if got := presignReq.Files; len(got) != 2 || got[0] != filepath.Base(pkg) || got[1] != filepath.Base(sig) {
		t.Fatalf("presign files = %v", got)
	}
	if stored[filepath.Base(pkg)] != "PKGBYTES" || stored[filepath.Base(sig)] != "SIGBYTES" {
		t.Fatalf("stored = %v", stored)
	}
	if !finalized {
		t.Fatal("finalize was not called")
	}
	if got := finalizeReq.Packages; len(got) != 1 || got[0] != filepath.Base(pkg) {
		t.Fatalf("finalize packages = %v (should exclude .sig)", got)
	}
}

func TestUploadPackageFilesMultipartFallback(t *testing.T) {
	dir := t.TempDir()
	pkg := filepath.Join(dir, "bar-2-1-x86_64.pkg.tar.zst")
	sig := pkg + ".sig"
	writeFile(t, pkg, "PKG2")
	writeFile(t, sig, "SIG2")

	var gotPackage, gotSignature string
	var multipartHit bool
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/packages/presign"):
			w.WriteHeader(http.StatusNotImplemented)
			_ = json.NewEncoder(w).Encode(map[string]string{"message": "unsupported"})
		case strings.HasSuffix(r.URL.Path, "/packages"):
			multipartHit = true
			if err := r.ParseMultipartForm(1 << 20); err != nil {
				t.Errorf("parse multipart: %v", err)
			}
			if fhs := r.MultipartForm.File["package"]; len(fhs) == 1 {
				f, _ := fhs[0].Open()
				b, _ := io.ReadAll(f)
				gotPackage = fhs[0].Filename + ":" + string(b)
			}
			if fhs := r.MultipartForm.File["signature"]; len(fhs) == 1 {
				f, _ := fhs[0].Open()
				b, _ := io.ReadAll(f)
				gotSignature = fhs[0].Filename + ":" + string(b)
			}
			w.WriteHeader(http.StatusOK)
		default:
			t.Errorf("unexpected api path %q", r.URL.Path)
			w.WriteHeader(http.StatusBadRequest)
		}
	}))
	defer api.Close()

	if err := UploadPackageFiles(context.Background(), api.URL, "tok", "myrepo", pkg, sig); err != nil {
		t.Fatalf("UploadPackageFiles: %v", err)
	}
	if !multipartHit {
		t.Fatal("multipart endpoint was not called")
	}
	if want := filepath.Base(pkg) + ":PKG2"; gotPackage != want {
		t.Fatalf("package part = %q, want %q", gotPackage, want)
	}
	if want := filepath.Base(sig) + ":SIG2"; gotSignature != want {
		t.Fatalf("signature part = %q, want %q", gotSignature, want)
	}
}

func TestUploadPackageFilesNoPackages(t *testing.T) {
	if err := UploadPackageFiles(context.Background(), "http://example", "tok", "r", "only.sig"); err == nil {
		t.Fatal("expected an error when no package files are given")
	}
}
