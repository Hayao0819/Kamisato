package handler

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/mock/gomock"

	"github.com/Hayao0819/Kamisato/ayato/domain"
	"github.com/Hayao0819/Kamisato/ayato/platform"
	"github.com/Hayao0819/Kamisato/ayato/test/mocks"
	"github.com/Hayao0819/Kamisato/internal/conf"
)

func TestRepoFileHandlerRedirectsToPresignedURL(t *testing.T) {
	controller, service, handlers := setup(t)
	defer controller.Finish()

	const want = "https://s3.example.com/foo.pkg.tar.zst?sig=abc"
	service.EXPECT().
		SignedURL("myrepo", "x86_64", "foo.pkg.tar.zst").
		Return(want, nil)

	router := gin.New()
	router.GET("/repo/:repo/:arch/:file", handlers.Repositories.RepoFileHandler)
	response := httptest.NewRecorder()
	router.ServeHTTP(
		response,
		httptest.NewRequest(
			http.MethodGet,
			"/repo/myrepo/x86_64/foo.pkg.tar.zst",
			nil,
		),
	)
	if response.Code != http.StatusFound {
		t.Fatalf("status = %d, want 302: %s", response.Code, response.Body.String())
	}
	if location := response.Header().Get("Location"); location != want {
		t.Errorf("Location = %q, want %q", location, want)
	}
}

func TestRepoFileHandlerStreamsWhenPresignUnavailable(t *testing.T) {
	controller, service, handlers := setup(t)
	defer controller.Finish()

	const body = "package-bytes"
	file := platform.NewFileStream(
		"foo.pkg.tar.zst",
		"application/octet-stream",
		bufferToReadSeekCloser(bytes.NewBufferString(body)),
	)
	service.EXPECT().
		SignedURL("myrepo", "x86_64", "foo.pkg.tar.zst").
		Return("", nil)
	service.EXPECT().
		GetFileWithMeta("myrepo", "x86_64", "foo.pkg.tar.zst").
		Return(file, domain.FileMeta{}, nil)

	router := gin.New()
	router.GET("/repo/:repo/:arch/:file", handlers.Repositories.RepoFileHandler)
	response := httptest.NewRecorder()
	router.ServeHTTP(
		response,
		httptest.NewRequest(
			http.MethodGet,
			"/repo/myrepo/x86_64/foo.pkg.tar.zst",
			nil,
		),
	)
	if response.Code != http.StatusOK || response.Body.String() != body {
		t.Fatalf(
			"response = %d %q, want 200 %q",
			response.Code,
			response.Body.String(),
			body,
		)
	}
}

func TestRepoFileHandlerReturns304OnMatchingETag(t *testing.T) {
	controller, service, handlers := setup(t)
	defer controller.Finish()

	const body, etag = "db-bytes", `"v1"`
	expectRepositoryFile(
		service,
		"myrepo",
		"x86_64",
		"myrepo.db",
		body,
		domain.FileMeta{ETag: etag},
		2,
	)
	router := gin.New()
	router.GET("/repo/:repo/:arch/:file", handlers.Repositories.RepoFileHandler)

	response := httptest.NewRecorder()
	router.ServeHTTP(
		response,
		httptest.NewRequest(http.MethodGet, "/repo/myrepo/x86_64/myrepo.db", nil),
	)
	if response.Code != http.StatusOK || response.Header().Get("ETag") != etag {
		t.Fatalf("initial response = %d ETag %q", response.Code, response.Header().Get("ETag"))
	}

	response = httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/repo/myrepo/x86_64/myrepo.db", nil)
	request.Header.Set("If-None-Match", etag)
	router.ServeHTTP(response, request)
	if response.Code != http.StatusNotModified || response.Body.Len() != 0 {
		t.Fatalf("conditional response = %d %q, want empty 304", response.Code, response.Body.String())
	}
}

func TestRepoFileHandlerReturns304OnIfModifiedSince(t *testing.T) {
	controller, service, handlers := setup(t)
	defer controller.Finish()

	const body = "db-bytes"
	modified := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	expectRepositoryFile(
		service,
		"myrepo",
		"x86_64",
		"myrepo.db",
		body,
		domain.FileMeta{LastModified: modified},
		3,
	)
	router := gin.New()
	router.GET("/repo/:repo/:arch/:file", handlers.Repositories.RepoFileHandler)
	fetch := func(ifModifiedSince string) *httptest.ResponseRecorder {
		response := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, "/repo/myrepo/x86_64/myrepo.db", nil)
		if ifModifiedSince != "" {
			request.Header.Set("If-Modified-Since", ifModifiedSince)
		}
		router.ServeHTTP(response, request)
		return response
	}

	response := fetch("")
	if response.Code != http.StatusOK ||
		response.Header().Get("Last-Modified") != modified.Format(http.TimeFormat) {
		t.Fatalf("initial response = %d Last-Modified %q", response.Code, response.Header().Get("Last-Modified"))
	}
	if response = fetch(modified.Format(http.TimeFormat)); response.Code != http.StatusNotModified ||
		response.Body.Len() != 0 {
		t.Fatalf("current response = %d %q, want empty 304", response.Code, response.Body.String())
	}
	if response = fetch(modified.Add(-time.Hour).Format(http.TimeFormat)); response.Code != http.StatusOK ||
		response.Body.String() != body {
		t.Fatalf("stale response = %d %q, want 200 %q", response.Code, response.Body.String(), body)
	}
}

func TestRepoFileHandlerStreamsWhenRedirectDisabled(t *testing.T) {
	gin.SetMode(gin.TestMode)
	controller := gomock.NewController(t)
	defer controller.Finish()
	service := mocks.NewMockServicer(controller)

	const body = "package-bytes"
	file := platform.NewFileStream(
		"foo.pkg.tar.zst",
		"application/octet-stream",
		bufferToReadSeekCloser(bytes.NewBufferString(body)),
	)
	disabled := false
	handlers := New(service, &conf.AyatoConfig{RedirectDownloads: &disabled})
	service.EXPECT().
		GetFileWithMeta("myrepo", "x86_64", "foo.pkg.tar.zst").
		Return(file, domain.FileMeta{}, nil)

	router := gin.New()
	router.GET("/repo/:repo/:arch/:file", handlers.Repositories.RepoFileHandler)
	response := httptest.NewRecorder()
	router.ServeHTTP(
		response,
		httptest.NewRequest(
			http.MethodGet,
			"/repo/myrepo/x86_64/foo.pkg.tar.zst",
			nil,
		),
	)
	if response.Code != http.StatusOK || response.Body.String() != body {
		t.Fatalf(
			"response = %d %q, want 200 %q",
			response.Code,
			response.Body.String(),
			body,
		)
	}
}
