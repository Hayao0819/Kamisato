package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Hayao0819/Kamisato/ayato/domain"
	"github.com/Hayao0819/Kamisato/ayato/test/mocks"
	"github.com/gin-gonic/gin"
	"go.uber.org/mock/gomock"
)

func setup(t *testing.T) (*gomock.Controller, *mocks.MockServicer, *Handler) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	ctrl := gomock.NewController(t)
	mockSvc := mocks.NewMockServicer(ctrl)
	return ctrl, mockSvc, New(mockSvc, nil)
}

func TestHelloHandler(t *testing.T) {
	_, _, h := setup(t)
	r := gin.New()
	r.GET("/hello", h.HelloHandler)

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
	r.GET("/teapot", h.TeapotHandler)

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
	r.GET("/repos", h.ReposHandler)

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
	r.GET("/repos", h.ReposHandler)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/repos", nil))

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", w.Code)
	}
}

func TestArchesHandler(t *testing.T) {
	ctrl, mockSvc, h := setup(t)
	defer ctrl.Finish()

	mockSvc.EXPECT().Arches("myrepo").Return([]string{"x86_64"}, nil)

	r := gin.New()
	r.GET("/repos/:repo/archs", h.ArchesHandler)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/repos/myrepo/archs", nil))

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "x86_64") {
		t.Errorf("body = %q, want to contain x86_64", w.Body.String())
	}
}

func TestBlinkyRemoveHandler(t *testing.T) {
	ctrl, mockSvc, h := setup(t)
	defer ctrl.Finish()

	// The blinky route carries no arch, so the handler passes "" — "remove from
	// every arch that lists the package".
	mockSvc.EXPECT().RemovePkg("myrepo", "", "mypkg").Return(nil)

	r := gin.New()
	r.DELETE("/:repo/package/:name", h.BlinkyRemoveHandler)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodDelete, "/myrepo/package/mypkg", nil))

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "removed") {
		t.Errorf("body = %q, want to contain removed", w.Body.String())
	}
}

func TestRemoveHandlerExplicitArch(t *testing.T) {
	ctrl, mockSvc, h := setup(t)
	defer ctrl.Finish()

	mockSvc.EXPECT().RemovePkg("myrepo", "aarch64", "mypkg").Return(nil)

	r := gin.New()
	r.DELETE("/:repo/package/:name", h.BlinkyRemoveHandler)
	r.DELETE("/:repo/:arch/package/:name", h.BlinkyRemoveHandler)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodDelete, "/myrepo/aarch64/package/mypkg", nil))

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "removed") {
		t.Errorf("body = %q, want to contain removed", w.Body.String())
	}
}

func TestPkgFilesHandlerNotImplemented(t *testing.T) {
	ctrl, mockSvc, h := setup(t)
	defer ctrl.Finish()

	mockSvc.EXPECT().PkgFiles("myrepo", "x86_64", "mypkg").Return(nil, domain.ErrNotImplemented)

	r := gin.New()
	r.GET("/:repo/:arch/package/:name/files", h.PkgFilesHandler)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/myrepo/x86_64/package/mypkg/files", nil))

	if w.Code != http.StatusNotImplemented {
		t.Fatalf("status = %d, want 501: %s", w.Code, w.Body.String())
	}
}

var errTest = &testError{"boom"}

type testError struct{ msg string }

func (e *testError) Error() string { return e.msg }
