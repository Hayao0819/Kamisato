package handler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/mock/gomock"

	"github.com/Hayao0819/Kamisato/ayato/domain"
	"github.com/Hayao0819/Kamisato/ayato/httpapi"
	"github.com/Hayao0819/Kamisato/ayato/stream"
	"github.com/Hayao0819/Kamisato/ayato/test/mocks"
	"github.com/Hayao0819/Kamisato/internal/conf"
)

func TestHelloHandler(t *testing.T) {
	_, _, h := setup(t)
	r := gin.New()
	r.GET("/hello", h.System.HelloHandler)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/hello", nil))

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	if !strings.Contains(w.Body.String(), "Hello, Ayato!") {
		t.Errorf("body = %q, want to contain greeting", w.Body.String())
	}
}

func TestTeapotHandler(t *testing.T) {
	_, _, h := setup(t)
	r := gin.New()
	r.GET("/teapot", h.System.TeapotHandler)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/teapot", nil))

	if w.Code != http.StatusTeapot {
		t.Fatalf("status = %d, want 418", w.Code)
	}
}

func TestReposHandler(t *testing.T) {
	ctrl, mockSvc, h := setup(t)
	defer ctrl.Finish()

	mockSvc.EXPECT().RepoNames().Return([]string{"core", "extra"}, nil)

	r := gin.New()
	r.GET("/repos", h.Repositories.ReposHandler)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/repos", nil))

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", w.Code, w.Body.String())
	}
	if body := w.Body.String(); !strings.Contains(body, "core") || !strings.Contains(body, "extra") {
		t.Errorf("body = %q, want to contain core and extra", body)
	}
}

func TestReposHandler_Error(t *testing.T) {
	ctrl, mockSvc, h := setup(t)
	defer ctrl.Finish()

	mockSvc.EXPECT().RepoNames().Return(nil, errTest)

	r := gin.New()
	r.GET("/repos", h.Repositories.ReposHandler)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/repos", nil))

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", w.Code)
	}
}

func TestRepoDetailHandler(t *testing.T) {
	ctrl, mockSvc, h := setup(t)
	defer ctrl.Finish()

	mockSvc.EXPECT().Arches("myrepo").Return([]string{"x86_64"}, nil)

	r := gin.New()
	r.GET("/repos/:repo", h.Repositories.RepoDetailHandler)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/repos/myrepo", nil))

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "x86_64") {
		t.Errorf("body = %q, want to contain x86_64", w.Body.String())
	}
}

func TestBlinkyRemoveHandler(t *testing.T) {
	for _, test := range []struct {
		name, route, path, arch string
	}{
		{
			name:  "blinky removes from every arch",
			route: "/:repo/package/:name",
			path:  "/myrepo/package/mypkg",
		},
		{
			name:  "management route uses explicit arch",
			route: "/repos/:repo/:arch/packages/:name",
			path:  "/repos/myrepo/aarch64/packages/mypkg",
			arch:  "aarch64",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			controller, service, handlers := setup(t)
			defer controller.Finish()
			service.EXPECT().RemovePkg("myrepo", test.arch, "mypkg").Return(nil)

			router := gin.New()
			router.DELETE(test.route, handlers.Publications.BlinkyRemoveHandler)
			response := httptest.NewRecorder()
			router.ServeHTTP(
				response,
				httptest.NewRequest(http.MethodDelete, test.path, nil),
			)
			if response.Code != http.StatusOK {
				t.Fatalf(
					"status = %d, want 200: %s",
					response.Code,
					response.Body.String(),
				)
			}
			if !strings.Contains(response.Body.String(), "removed") {
				t.Errorf("body = %q, want removed", response.Body.String())
			}
		})
	}
}

func TestPkgFilesHandlerNotImplemented(t *testing.T) {
	ctrl, mockSvc, h := setup(t)
	defer ctrl.Finish()

	mockSvc.EXPECT().PkgFiles("myrepo", "x86_64", "mypkg").Return(nil, domain.ErrNotImplemented)

	r := gin.New()
	r.GET("/repos/:repo/:arch/packages/:name/files", h.Repositories.PkgFilesHandler)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/repos/myrepo/x86_64/packages/mypkg/files", nil))

	if w.Code != http.StatusNotImplemented {
		t.Fatalf("status = %d, want 501: %s", w.Code, w.Body.String())
	}
	var response httpapi.ErrorResponse
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.Code != httpapi.CodeNotImplemented {
		t.Errorf("code = %q, want %q", response.Code, httpapi.CodeNotImplemented)
	}
}

func TestPkgFilesHandlerNotFound(t *testing.T) {
	ctrl, mockSvc, h := setup(t)
	defer ctrl.Finish()

	mockSvc.EXPECT().PkgFiles("myrepo", "x86_64", "missing").
		Return(nil, fmt.Errorf("%w: private storage detail", domain.ErrNotFound))

	r := gin.New()
	r.GET("/repos/:repo/:arch/packages/:name/files", h.Repositories.PkgFilesHandler)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/repos/myrepo/x86_64/packages/missing/files", nil))

	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404: %s", w.Code, w.Body.String())
	}
	var response httpapi.ErrorResponse
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.Code != httpapi.CodeNotFound || strings.Contains(w.Body.String(), "private storage detail") {
		t.Errorf("response = %s, want safe not_found envelope", w.Body.String())
	}
}

func TestRepoFileHandlerRedirectsToPresignedURL(t *testing.T) {
	ctrl, mockSvc, h := setup(t) // nil cfg defaults to redirect-on
	defer ctrl.Finish()

	const want = "https://s3.example.com/foo.pkg.tar.zst?sig=abc"
	mockSvc.EXPECT().SignedURL("myrepo", "x86_64", "foo.pkg.tar.zst").Return(want, nil)

	r := gin.New()
	r.GET("/repo/:repo/:arch/:file", h.Repositories.RepoFileHandler)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/repo/myrepo/x86_64/foo.pkg.tar.zst", nil))

	if w.Code != http.StatusFound {
		t.Fatalf("status = %d, want 302: %s", w.Code, w.Body.String())
	}
	if loc := w.Header().Get("Location"); loc != want {
		t.Errorf("Location = %q, want %q", loc, want)
	}
}

func TestRepoFileHandlerStreamsWhenPresignUnavailable(t *testing.T) {
	ctrl, mockSvc, h := setup(t)
	defer ctrl.Finish()

	const body = "package-bytes"
	fs := stream.NewFileStream("foo.pkg.tar.zst", "application/octet-stream",
		bufferToReadSeekCloser(bytes.NewBufferString(body)))

	// localfs cannot presign: SignedURL returns "", so the handler streams.
	mockSvc.EXPECT().SignedURL("myrepo", "x86_64", "foo.pkg.tar.zst").Return("", nil)
	mockSvc.EXPECT().GetFileWithMeta("myrepo", "x86_64", "foo.pkg.tar.zst").Return(fs, domain.FileMeta{}, nil)

	r := gin.New()
	r.GET("/repo/:repo/:arch/:file", h.Repositories.RepoFileHandler)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/repo/myrepo/x86_64/foo.pkg.tar.zst", nil))

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", w.Code, w.Body.String())
	}
	if w.Body.String() != body {
		t.Errorf("body = %q, want %q", w.Body.String(), body)
	}
}

// A backend that returns a version token surfaces it as a strong ETag; a matching
// If-None-Match then yields a bodyless 304 so pacman skips the re-download.
func TestRepoFileHandlerReturns304OnMatchingETag(t *testing.T) {
	ctrl, mockSvc, h := setup(t)
	defer ctrl.Finish()

	const body, etag = "db-bytes", `"v1"`
	expectRepositoryFile(
		mockSvc,
		"myrepo",
		"x86_64",
		"myrepo.db",
		body,
		domain.FileMeta{ETag: etag},
		2,
	)

	r := gin.New()
	r.GET("/repo/:repo/:arch/:file", h.Repositories.RepoFileHandler)

	// First fetch: 200 with the ETag advertised.
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/repo/myrepo/x86_64/myrepo.db", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("first status = %d, want 200", w.Code)
	}
	if got := w.Header().Get("ETag"); got != etag {
		t.Fatalf("ETag = %q, want %q", got, etag)
	}

	// Conditional re-fetch with the same validator: 304, no body.
	w = httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/repo/myrepo/x86_64/myrepo.db", nil)
	req.Header.Set("If-None-Match", etag)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusNotModified {
		t.Fatalf("conditional status = %d, want 304", w.Code)
	}
	if w.Body.Len() != 0 {
		t.Errorf("304 body = %q, want empty", w.Body.String())
	}
}

// pacman drives conditional downloads off Last-Modified/If-Modified-Since (not
// ETag), so a request whose If-Modified-Since is at or after the file's mtime
// must get a bodyless 304, and an older one a full 200.
func TestRepoFileHandlerReturns304OnIfModifiedSince(t *testing.T) {
	ctrl, mockSvc, h := setup(t)
	defer ctrl.Finish()

	const body = "db-bytes"
	modtime := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	expectRepositoryFile(
		mockSvc,
		"myrepo",
		"x86_64",
		"myrepo.db",
		body,
		domain.FileMeta{LastModified: modtime},
		3,
	)

	r := gin.New()
	r.GET("/repo/:repo/:arch/:file", h.Repositories.RepoFileHandler)

	do := func(ifModifiedSince string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/repo/myrepo/x86_64/myrepo.db", nil)
		if ifModifiedSince != "" {
			req.Header.Set("If-Modified-Since", ifModifiedSince)
		}
		r.ServeHTTP(w, req)
		return w
	}

	// Unconditional: 200 with Last-Modified advertised.
	w := do("")
	if w.Code != http.StatusOK {
		t.Fatalf("unconditional status = %d, want 200", w.Code)
	}
	if got := w.Header().Get("Last-Modified"); got != modtime.Format(http.TimeFormat) {
		t.Fatalf("Last-Modified = %q, want %q", got, modtime.Format(http.TimeFormat))
	}

	// Client's copy is as new as the file: 304, no body.
	if w := do(modtime.Format(http.TimeFormat)); w.Code != http.StatusNotModified || w.Body.Len() != 0 {
		t.Fatalf("If-Modified-Since==mtime: status=%d body=%q, want 304 empty", w.Code, w.Body.String())
	}

	// Client's copy is older than the file: full 200.
	if w := do(modtime.Add(-time.Hour).Format(http.TimeFormat)); w.Code != http.StatusOK || w.Body.String() != body {
		t.Fatalf("older If-Modified-Since: status=%d, want 200 with body", w.Code)
	}
}

func TestRepoFileHandlerStreamsWhenRedirectDisabled(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockSvc := mocks.NewMockServicer(ctrl)

	const body = "package-bytes"
	fs := stream.NewFileStream("foo.pkg.tar.zst", "application/octet-stream",
		bufferToReadSeekCloser(bytes.NewBufferString(body)))

	// redirect_downloads=false forces streaming, so SignedURL is never consulted.
	disabled := false
	h := New(mockSvc, &conf.AyatoConfig{RedirectDownloads: &disabled})
	mockSvc.EXPECT().GetFileWithMeta("myrepo", "x86_64", "foo.pkg.tar.zst").Return(fs, domain.FileMeta{}, nil)

	r := gin.New()
	r.GET("/repo/:repo/:arch/:file", h.Repositories.RepoFileHandler)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/repo/myrepo/x86_64/foo.pkg.tar.zst", nil))

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", w.Code, w.Body.String())
	}
	if w.Body.String() != body {
		t.Errorf("body = %q, want %q", w.Body.String(), body)
	}
}
