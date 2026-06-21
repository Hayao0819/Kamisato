package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Hayao0819/Kamisato/miko/service"
	"github.com/Hayao0819/Kamisato/miko/test/mocks"
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

func TestSubmitBuildHandlerSuccess(t *testing.T) {
	ctrl, mockSvc, h := setup(t)
	defer ctrl.Finish()

	mockSvc.EXPECT().Submit(gomock.Any()).Return("job123", nil)

	r := gin.New()
	r.POST("/build", h.SubmitBuildHandler)

	body := strings.NewReader(`{"repo":"extra","arch":"x86_64","pkgbuild":"pkgname=foo"}`)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/build", body))

	if w.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want 202: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "job123") {
		t.Errorf("body = %q, want to contain job id", w.Body.String())
	}
}

func TestSubmitBuildHandlerInvalidJSON(t *testing.T) {
	ctrl, _, h := setup(t)
	defer ctrl.Finish()

	r := gin.New()
	r.POST("/build", h.SubmitBuildHandler)

	body := strings.NewReader(`{not json`)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/build", body))

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400: %s", w.Code, w.Body.String())
	}
}

func TestJobCancelHandlerOK(t *testing.T) {
	ctrl, mockSvc, h := setup(t)
	defer ctrl.Finish()

	mockSvc.EXPECT().Cancel("job123").Return(nil)

	r := gin.New()
	r.DELETE("/jobs/:id", h.JobCancelHandler)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodDelete, "/jobs/job123", nil))

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "cancelled") {
		t.Errorf("body = %q, want to contain cancelled", w.Body.String())
	}
}

func TestJobCancelHandlerNotFound(t *testing.T) {
	ctrl, mockSvc, h := setup(t)
	defer ctrl.Finish()

	mockSvc.EXPECT().Cancel("missing").Return(service.ErrInvalidRequest)

	r := gin.New()
	r.DELETE("/jobs/:id", h.JobCancelHandler)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodDelete, "/jobs/missing", nil))

	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404: %s", w.Code, w.Body.String())
	}
}

func TestJobCancelHandlerConflict(t *testing.T) {
	ctrl, mockSvc, h := setup(t)
	defer ctrl.Finish()

	mockSvc.EXPECT().Cancel("done").Return(service.ErrJobNotCancelable)

	r := gin.New()
	r.DELETE("/jobs/:id", h.JobCancelHandler)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodDelete, "/jobs/done", nil))

	if w.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409: %s", w.Code, w.Body.String())
	}
}
