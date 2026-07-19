package handler

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"go.uber.org/mock/gomock"

	"github.com/Hayao0819/Kamisato/ayato/domain"
	"github.com/Hayao0819/Kamisato/ayato/test/mocks"
	"github.com/Hayao0819/Kamisato/internal/conf"
)

func TestUnsafePresignProtocolIsDisabled(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := &PublicationHandler{}

	for _, tc := range []struct {
		name, path string
		handler    gin.HandlerFunc
	}{
		{name: "presign", path: "/api/unstable/repos/core/packages/presign", handler: h.PresignUploadHandler},
		{name: "finalize", path: "/api/unstable/repos/core/packages/finalize", handler: h.FinalizeUploadHandler},
	} {
		t.Run(tc.name, func(t *testing.T) {
			router := gin.New()
			router.POST(tc.path, tc.handler)
			response := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodPost, tc.path, nil)
			router.ServeHTTP(response, request)
			if response.Code != http.StatusNotImplemented {
				t.Fatalf("status = %d, want 501", response.Code)
			}
		})
	}
}

func TestBatchUploadAllowsAggregateLargerThanOnePackageLimit(t *testing.T) {
	gin.SetMode(gin.TestMode)
	controller := gomock.NewController(t)
	service := mocks.NewMockServicer(controller)
	service.EXPECT().UploadFiles("core", gomock.Any()).DoAndReturn(func(_ string, files []*domain.UploadFiles) error {
		if len(files) != 2 {
			t.Fatalf("uploaded files = %d, want 2", len(files))
		}
		return nil
	})
	cfg := &conf.AyatoConfig{MaxSize: 8, MaxBatchPackages: 2, MaxBatchBytes: 32}
	h := NewPublicationHandler(service, service, service, cfg)

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	for _, name := range []string{"foo.pkg.tar.zst", "foo-docs.pkg.tar.zst"} {
		part, err := writer.CreateFormFile("package", name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := part.Write([]byte("123456")); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}

	router := gin.New()
	router.POST("/api/unstable/repos/:repo/packages", h.BatchUploadHandler)
	request := httptest.NewRequest(http.MethodPost, "/api/unstable/repos/core/packages", &body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
}
