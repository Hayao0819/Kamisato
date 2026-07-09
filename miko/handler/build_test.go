package handler_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"go.uber.org/mock/gomock"

	"github.com/Hayao0819/Kamisato/internal/auth/apikey"
	"github.com/Hayao0819/Kamisato/internal/conf"
	"github.com/Hayao0819/Kamisato/miko/handler"
	"github.com/Hayao0819/Kamisato/miko/router"
	"github.com/Hayao0819/Kamisato/miko/service"
	"github.com/Hayao0819/Kamisato/miko/test/mocks"
)

// setup wires the mock service through the production router so tests exercise
// the same /api/unstable paths (and middleware) the server serves.
func setup(t *testing.T) (*gomock.Controller, *mocks.MockServicer, http.Handler) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	ctrl := gomock.NewController(t)
	mockSvc := mocks.NewMockServicer(ctrl)
	h := handler.New(mockSvc, &conf.MikoConfig{})

	e := gin.New()
	if err := router.SetRoute(e, h, apikey.NewVerifier(nil)); err != nil {
		t.Fatalf("SetRoute: %v", err)
	}
	return ctrl, mockSvc, e
}

func TestSubmitBuildHandlerSuccess(t *testing.T) {
	ctrl, mockSvc, r := setup(t)
	defer ctrl.Finish()

	mockSvc.EXPECT().Submit(gomock.Any()).Return("job123", nil)

	body := strings.NewReader(`{"repo":"extra","arch":"x86_64","pkgbuild":"pkgname=foo"}`)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/api/unstable/build", body))

	if w.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want 202: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "job123") {
		t.Errorf("body = %q, want to contain job id", w.Body.String())
	}
}

func TestSubmitBuildHandlerInvalidJSON(t *testing.T) {
	ctrl, _, r := setup(t)
	defer ctrl.Finish()

	body := strings.NewReader(`{not json`)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/api/unstable/build", body))

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400: %s", w.Code, w.Body.String())
	}
}

func TestJobCancelHandlerOK(t *testing.T) {
	ctrl, mockSvc, r := setup(t)
	defer ctrl.Finish()

	mockSvc.EXPECT().Cancel("job123").Return(nil)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodDelete, "/api/unstable/jobs/job123", nil))

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "cancelled") {
		t.Errorf("body = %q, want to contain cancelled", w.Body.String())
	}
}

func TestJobCancelHandlerNotFound(t *testing.T) {
	ctrl, mockSvc, r := setup(t)
	defer ctrl.Finish()

	mockSvc.EXPECT().Cancel("missing").Return(service.ErrInvalidRequest)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodDelete, "/api/unstable/jobs/missing", nil))

	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404: %s", w.Code, w.Body.String())
	}
}

func TestJobCancelHandlerConflict(t *testing.T) {
	ctrl, mockSvc, r := setup(t)
	defer ctrl.Finish()

	mockSvc.EXPECT().Cancel("done").Return(service.ErrJobNotCancelable)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodDelete, "/api/unstable/jobs/done", nil))

	if w.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409: %s", w.Code, w.Body.String())
	}
}
