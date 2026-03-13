package handler

import (
	"bytes"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Hayao0819/Kamisato/ayato/domain"
	"github.com/Hayao0819/Kamisato/ayato/test/mocks"
	"github.com/gin-gonic/gin"
	"go.uber.org/mock/gomock"
)

func setupBuildTest(t *testing.T) (*gomock.Controller, *mocks.MockIService, *gin.Engine) {
	t.Helper()
	ctrl := gomock.NewController(t)
	mockSvc := mocks.NewMockIService(ctrl)
	h := New(mockSvc, nil)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/api/build/:repo", h.BuildPackageHandler)
	return ctrl, mockSvc, r
}

func createBuildForm(t *testing.T, pkgbuild string, arch string, gpgkey string) (*bytes.Buffer, string) {
	t.Helper()
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	if pkgbuild != "" {
		part, err := writer.CreateFormFile("pkgbuild", "PKGBUILD")
		if err != nil {
			t.Fatal(err)
		}
		part.Write([]byte(pkgbuild))
	}

	if arch != "" {
		writer.WriteField("arch", arch)
	}
	if gpgkey != "" {
		writer.WriteField("gpgkey", gpgkey)
	}

	writer.Close()
	return body, writer.FormDataContentType()
}

func TestBuildPackageHandler_Success(t *testing.T) {
	ctrl, mockSvc, r := setupBuildTest(t)
	defer ctrl.Finish()

	mockSvc.EXPECT().
		BuildPackage("myrepo", gomock.Any()).
		DoAndReturn(func(repo string, req *domain.BuildRequest) error {
			if req.Arch != "aarch64" {
				t.Errorf("expected arch aarch64, got %s", req.Arch)
			}
			return nil
		})

	body, contentType := createBuildForm(t, "pkgname=test\npkgver=1.0", "aarch64", "")

	req := httptest.NewRequest(http.MethodPost, "/api/build/myrepo", body)
	req.Header.Set("Content-Type", contentType)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d: %s", http.StatusOK, w.Code, w.Body.String())
	}
}

func TestBuildPackageHandler_DefaultArch(t *testing.T) {
	ctrl, mockSvc, r := setupBuildTest(t)
	defer ctrl.Finish()

	mockSvc.EXPECT().
		BuildPackage("myrepo", gomock.Any()).
		DoAndReturn(func(repo string, req *domain.BuildRequest) error {
			if req.Arch != "x86_64" {
				t.Errorf("expected default arch x86_64, got %s", req.Arch)
			}
			return nil
		})

	body, contentType := createBuildForm(t, "pkgname=test\npkgver=1.0", "", "")

	req := httptest.NewRequest(http.MethodPost, "/api/build/myrepo", body)
	req.Header.Set("Content-Type", contentType)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d: %s", http.StatusOK, w.Code, w.Body.String())
	}
}

func TestBuildPackageHandler_NoPKGBUILD(t *testing.T) {
	ctrl, _, r := setupBuildTest(t)
	defer ctrl.Finish()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	writer.WriteField("arch", "x86_64")
	writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/build/myrepo", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d: %s", http.StatusBadRequest, w.Code, w.Body.String())
	}
}

func TestBuildPackageHandler_NoRepoName(t *testing.T) {
	ctrl, _, _ := setupBuildTest(t)
	defer ctrl.Finish()

	// Create a router without :repo param to simulate empty repo
	mockSvc := mocks.NewMockIService(ctrl)
	h := New(mockSvc, nil)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/api/build/", h.BuildPackageHandler)

	body, contentType := createBuildForm(t, "pkgname=test", "x86_64", "")

	req := httptest.NewRequest(http.MethodPost, "/api/build/", body)
	req.Header.Set("Content-Type", contentType)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d: %s", http.StatusBadRequest, w.Code, w.Body.String())
	}
}

func TestBuildPackageHandler_BuildError(t *testing.T) {
	ctrl, mockSvc, r := setupBuildTest(t)
	defer ctrl.Finish()

	mockSvc.EXPECT().
		BuildPackage("myrepo", gomock.Any()).
		Return(fmt.Errorf("docker build failed"))

	body, contentType := createBuildForm(t, "pkgname=test\npkgver=1.0", "x86_64", "")

	req := httptest.NewRequest(http.MethodPost, "/api/build/myrepo", body)
	req.Header.Set("Content-Type", contentType)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status %d, got %d: %s", http.StatusInternalServerError, w.Code, w.Body.String())
	}
}
