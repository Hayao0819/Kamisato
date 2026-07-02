package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Hayao0819/Kamisato/ayato/domain"
	"github.com/gin-gonic/gin"
)

func TestAdminsRemoveHandlerRejectsLastAdmin(t *testing.T) {
	ctrl, mockSvc, h := setup(t)
	defer ctrl.Finish()

	// Only one admin left, and it is the one being removed: refuse so the
	// allowlist never empties and locks everyone out.
	mockSvc.EXPECT().ListAdmins().Return([]domain.AllowedAdmin{{ID: 42}}, nil)

	r := gin.New()
	r.DELETE("/auth/admins/:id", h.AdminsRemoveHandler)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodDelete, "/auth/admins/42", nil))

	if w.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409: %s", w.Code, w.Body.String())
	}
}

func TestAdminsRemoveHandlerAllowsWhenOthersRemain(t *testing.T) {
	ctrl, mockSvc, h := setup(t)
	defer ctrl.Finish()

	mockSvc.EXPECT().ListAdmins().Return([]domain.AllowedAdmin{{ID: 42}, {ID: 7}}, nil)
	mockSvc.EXPECT().RemoveAdmin(int64(42)).Return(nil)

	r := gin.New()
	r.DELETE("/auth/admins/:id", h.AdminsRemoveHandler)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodDelete, "/auth/admins/42", nil))

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", w.Code, w.Body.String())
	}
}

// Removing an id that is not the sole surviving admin is allowed even when only
// one admin remains, because the allowlist does not drop to zero.
func TestAdminsRemoveHandlerAllowsRemovingNonAdmin(t *testing.T) {
	ctrl, mockSvc, h := setup(t)
	defer ctrl.Finish()

	mockSvc.EXPECT().ListAdmins().Return([]domain.AllowedAdmin{{ID: 42}}, nil)
	mockSvc.EXPECT().RemoveAdmin(int64(7)).Return(nil)

	r := gin.New()
	r.DELETE("/auth/admins/:id", h.AdminsRemoveHandler)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodDelete, "/auth/admins/7", nil))

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", w.Code, w.Body.String())
	}
}
