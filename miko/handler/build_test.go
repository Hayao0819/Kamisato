package handler_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/mock/gomock"

	"github.com/Hayao0819/Kamisato/internal/auth/apikey"
	"github.com/Hayao0819/Kamisato/internal/conf"
	"github.com/Hayao0819/Kamisato/miko/domain"
	"github.com/Hayao0819/Kamisato/miko/handler"
	"github.com/Hayao0819/Kamisato/miko/router"
	"github.com/Hayao0819/Kamisato/miko/service"
	"github.com/Hayao0819/Kamisato/miko/test/mocks"
)

type spaceReader struct{}

func (spaceReader) Read(p []byte) (int, error) {
	for i := range p {
		p[i] = ' '
	}
	return len(p), nil
}

// setup wires the mock service through the production router so tests exercise
// the same /api/unstable paths (and middleware) the server serves.
func setup(t *testing.T) (*gomock.Controller, *mocks.MockServicer, http.Handler) {
	t.Helper()
	return setupWithVerifier(t, apikey.NewVerifier(nil))
}

func setupWithVerifier(t *testing.T, verifier *apikey.Verifier) (*gomock.Controller, *mocks.MockServicer, http.Handler) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	ctrl := gomock.NewController(t)
	mockSvc := mocks.NewMockServicer(ctrl)
	h := handler.New(mockSvc, &conf.MikoConfig{})

	e := gin.New()
	if err := router.SetRoute(e, h, verifier); err != nil {
		t.Fatalf("SetRoute: %v", err)
	}
	return ctrl, mockSvc, e
}

func scopedVerifier() *apikey.Verifier {
	return apikey.NewScopedVerifier([]apikey.Entry{
		{Name: "alice", Key: "alice-key", Scopes: []string{apikey.ScopeBuildSubmit, apikey.ScopeBuildRead, apikey.ScopeBuildCancel}},
		{Name: "alice-rotated", Principal: "alice", Key: "alice-new-key", Scopes: []string{apikey.ScopeBuildSubmit, apikey.ScopeBuildRead, apikey.ScopeBuildCancel}},
		{Name: "bob", Key: "bob-key", Scopes: []string{apikey.ScopeBuildSubmit, apikey.ScopeBuildRead, apikey.ScopeBuildCancel}},
		{Name: "proxy", Key: "proxy-key", Scopes: []string{apikey.ScopeBuildAdmin}},
	})
}

func TestRotatedKeyKeepsExistingJobOwnerIdentity(t *testing.T) {
	ctrl, mockSvc, router := setupWithVerifier(t, scopedVerifier())
	defer ctrl.Finish()
	mockSvc.EXPECT().List().Return([]*domain.BuildJob{
		{ID: "pre-rotation-job", Owner: "alice", Status: domain.JobStatusQueued, CreatedAt: time.Now()},
	})
	request := httptest.NewRequest(http.MethodGet, "/api/unstable/jobs", nil)
	request.Header.Set(apikey.Header, "alice-new-key")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "pre-rotation-job") {
		t.Fatalf("rotated key could not see stable-principal job: status=%d body=%s", response.Code, response.Body.String())
	}
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

func TestScopedPrincipalOwnsSubmittedJob(t *testing.T) {
	ctrl, mockSvc, router := setupWithVerifier(t, scopedVerifier())
	defer ctrl.Finish()

	mockSvc.EXPECT().Submit(gomock.Any()).DoAndReturn(func(request *domain.BuildRequest) (string, error) {
		if request.Requester != "alice" {
			t.Fatalf("requester = %q", request.Requester)
		}
		return "owned-job", nil
	})
	request := httptest.NewRequest(http.MethodPost, "/api/unstable/build", strings.NewReader(`{"arch":"x86_64","pkgbuild":"pkgname=foo"}`))
	request.Header.Set(apikey.Header, "alice-key")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusAccepted {
		t.Fatalf("status = %d: %s", response.Code, response.Body.String())
	}
}

func TestScopedPrincipalListsOnlyOwnedJobs(t *testing.T) {
	ctrl, mockSvc, router := setupWithVerifier(t, scopedVerifier())
	defer ctrl.Finish()
	mockSvc.EXPECT().List().Return([]*domain.BuildJob{
		{ID: "alice-job", Owner: "alice", Status: domain.JobStatusQueued, CreatedAt: time.Now()},
		{ID: "bob-job", Owner: "bob", Status: domain.JobStatusQueued, CreatedAt: time.Now()},
		{ID: "legacy-job", Status: domain.JobStatusQueued, CreatedAt: time.Now()},
	})
	request := httptest.NewRequest(http.MethodGet, "/api/unstable/jobs", nil)
	request.Header.Set(apikey.Header, "alice-key")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d", response.Code)
	}
	if body := response.Body.String(); !strings.Contains(body, "alice-job") || strings.Contains(body, "bob-job") || strings.Contains(body, "legacy-job") {
		t.Fatalf("filtered body = %s", body)
	}
}

func TestScopedPrincipalCannotReadAnotherOwnersJob(t *testing.T) {
	ctrl, mockSvc, router := setupWithVerifier(t, scopedVerifier())
	defer ctrl.Finish()
	mockSvc.EXPECT().Status("alice-job").Return(&domain.BuildJob{ID: "alice-job", Owner: "alice"}, nil)
	request := httptest.NewRequest(http.MethodGet, "/api/unstable/jobs/alice-job", nil)
	request.Header.Set(apikey.Header, "bob-key")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", response.Code)
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

func TestSubmitBuildHandlerRejectsOversizedJSON(t *testing.T) {
	ctrl, _, r := setup(t)
	defer ctrl.Finish()
	body := io.LimitReader(spaceReader{}, (32<<20)+1)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/api/unstable/build", body))
	if w.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want 413: %s", w.Code, w.Body.String())
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

func TestJobCancelHandlerErrors(t *testing.T) {
	cases := []struct {
		id     string
		err    error
		status int
	}{
		{"missing", service.ErrInvalidRequest, http.StatusNotFound},
		{"done", service.ErrJobNotCancelable, http.StatusConflict},
	}
	for _, tc := range cases {
		t.Run(tc.id, func(t *testing.T) {
			ctrl, mockSvc, r := setup(t)
			defer ctrl.Finish()
			mockSvc.EXPECT().Cancel(tc.id).Return(tc.err)

			w := httptest.NewRecorder()
			r.ServeHTTP(w, httptest.NewRequest(http.MethodDelete, "/api/unstable/jobs/"+tc.id, nil))
			if w.Code != tc.status {
				t.Fatalf("status = %d, want %d: %s", w.Code, tc.status, w.Body.String())
			}
		})
	}
}
