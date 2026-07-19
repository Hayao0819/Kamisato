package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/Hayao0819/Kamisato/ayato/domain"
	"github.com/Hayao0819/Kamisato/ayato/platform"
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
	var response platform.HTTPErrorResponse
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.Code != platform.HTTPCodeNotImplemented {
		t.Errorf("code = %q, want %q", response.Code, platform.HTTPCodeNotImplemented)
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
	var response platform.HTTPErrorResponse
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.Code != platform.HTTPCodeNotFound || strings.Contains(w.Body.String(), "private storage detail") {
		t.Errorf("response = %s, want safe not_found envelope", w.Body.String())
	}
}
