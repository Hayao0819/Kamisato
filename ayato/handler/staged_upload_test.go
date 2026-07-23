package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"go.uber.org/mock/gomock"

	"github.com/Hayao0819/Kamisato/ayato/domain"
	"github.com/Hayao0819/Kamisato/ayato/test/mocks"
	"github.com/Hayao0819/Kamisato/internal/conf"
)

func stagedUploadRouter(t *testing.T) (*gin.Engine, *mocks.MockServicer) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	service := mocks.NewMockServicer(gomock.NewController(t))
	h := NewPublicationHandler(service, service, service, &conf.AyatoConfig{})
	router := gin.New()
	router.POST("/api/unstable/repos/:repo/packages/presign", h.PresignUploadHandler)
	router.POST("/api/unstable/repos/:repo/packages/commit", h.CommitUploadHandler)
	return router, service
}

func postStagedJSON(router *gin.Engine, path, body string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	return response
}

func TestPresignUploadHandlerResponseShape(t *testing.T) {
	router, service := stagedUploadRouter(t)
	service.EXPECT().
		PresignUpload("core", []domain.StagedFileRequest{
			{Name: "foo-1.0-1-x86_64.pkg.tar.zst", Size: 42},
			{Name: "foo-1.0-1-x86_64.pkg.tar.zst.sig", Size: 3},
		}).
		Return(&domain.StagedUploadGrant{
			ID:         "0123456789abcdef",
			TTLSeconds: 3600,
			URLs: map[string]string{
				"foo-1.0-1-x86_64.pkg.tar.zst":     "https://storage.example/a",
				"foo-1.0-1-x86_64.pkg.tar.zst.sig": "https://storage.example/b",
			},
		}, nil)

	response := postStagedJSON(router, "/api/unstable/repos/core/packages/presign", `{
		"files": [
			{"name": "foo-1.0-1-x86_64.pkg.tar.zst", "size": 42},
			{"name": "foo-1.0-1-x86_64.pkg.tar.zst.sig", "size": 3}
		]
	}`)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	var body struct {
		ID         string            `json:"id"`
		TTLSeconds int               `json:"ttl_seconds"`
		URLs       map[string]string `json:"urls"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.ID != "0123456789abcdef" || body.TTLSeconds != 3600 || len(body.URLs) != 2 {
		t.Fatalf("response = %+v", body)
	}
	if body.URLs["foo-1.0-1-x86_64.pkg.tar.zst"] != "https://storage.example/a" {
		t.Fatalf("urls = %v", body.URLs)
	}
}

func TestCommitUploadHandler(t *testing.T) {
	router, service := stagedUploadRouter(t)
	service.EXPECT().
		CommitUpload("core", "0123456789abcdef", []domain.StagedCommitEntry{
			{Package: "foo-1.0-1-x86_64.pkg.tar.zst", Signature: "foo-1.0-1-x86_64.pkg.tar.zst.sig"},
		}).
		Return(nil)

	response := postStagedJSON(router, "/api/unstable/repos/core/packages/commit", `{
		"id": "0123456789abcdef",
		"files": [
			{"package": "foo-1.0-1-x86_64.pkg.tar.zst", "signature": "foo-1.0-1-x86_64.pkg.tar.zst.sig"}
		]
	}`)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
}

func TestStagedUploadHandlersKeepTombstoneWhenCapabilityAbsent(t *testing.T) {
	router, service := stagedUploadRouter(t)
	service.EXPECT().PresignUpload("core", gomock.Any()).Return(nil, domain.ErrNotImplemented)
	service.EXPECT().CommitUpload("core", gomock.Any(), gomock.Any()).Return(domain.ErrNotImplemented)

	for _, path := range []string{
		"/api/unstable/repos/core/packages/presign",
		"/api/unstable/repos/core/packages/commit",
	} {
		response := postStagedJSON(router, path, `{"id":"0123456789abcdef","files":[]}`)
		if response.Code != http.StatusNotImplemented {
			t.Fatalf("%s status = %d, want 501", path, response.Code)
		}
		if !strings.Contains(response.Body.String(), "staging-intent") {
			t.Fatalf("%s body = %s, want the tombstone message", path, response.Body.String())
		}
	}
}

func TestStagedUploadHandlersRejectInvalidRequests(t *testing.T) {
	router, service := stagedUploadRouter(t)
	service.EXPECT().
		PresignUpload("core", gomock.Any()).
		Return(nil, fmt.Errorf("%w: invalid staged file name", domain.ErrInvalidUpload))
	service.EXPECT().
		CommitUpload("core", gomock.Any(), gomock.Any()).
		Return(fmt.Errorf("%w: invalid staged upload id", domain.ErrInvalidUpload))

	for _, tc := range []struct{ name, path, body string }{
		{name: "presign bad name", path: "/api/unstable/repos/core/packages/presign", body: `{"files":[{"name":"evil.txt"}]}`},
		{name: "commit bad id", path: "/api/unstable/repos/core/packages/commit", body: `{"id":"../x","files":[{"package":"a.pkg.tar.zst"}]}`},
		{name: "presign malformed json", path: "/api/unstable/repos/core/packages/presign", body: `{`},
		{name: "commit malformed json", path: "/api/unstable/repos/core/packages/commit", body: `{`},
	} {
		response := postStagedJSON(router, tc.path, tc.body)
		if response.Code != http.StatusBadRequest {
			t.Fatalf("%s status = %d, want 400 (body %s)", tc.name, response.Code, response.Body.String())
		}
	}
}
