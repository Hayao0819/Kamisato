package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"go.uber.org/mock/gomock"

	"github.com/Hayao0819/Kamisato/ayato/domain"
	"github.com/Hayao0819/Kamisato/ayato/test/mocks"
)

func TestPromoteHandlerParsesKnownTiers(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctrl := gomock.NewController(t)
	svc := mocks.NewMockServicer(ctrl)
	svc.EXPECT().
		PromotePackage(gomock.Any(), "core", domain.TierStaging, domain.TierTesting, "foo", "1.0-1").
		Return(nil)

	router := gin.New()
	router.POST("/repo/:repo/promote", New(svc, nil).PromoteHandler)
	req := httptest.NewRequest(
		http.MethodPost,
		"/repo/core/promote",
		strings.NewReader(`{"from":"staging","to":"testing","pkgname":"foo","version":"1.0-1"}`),
	)
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", recorder.Code, recorder.Body)
	}
}

func TestPromoteHandlerRejectsUnknownTierBeforeService(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctrl := gomock.NewController(t)
	svc := mocks.NewMockServicer(ctrl)

	router := gin.New()
	router.POST("/repo/:repo/promote", New(svc, nil).PromoteHandler)
	req := httptest.NewRequest(
		http.MethodPost,
		"/repo/core/promote",
		strings.NewReader(`{"from":"preview","to":"stable","pkgname":"foo"}`),
	)
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400: %s", recorder.Code, recorder.Body)
	}
}
