package buildclient

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Hayao0819/Kamisato/internal/errors"
)

// refreshServer serves a protected endpoint that rejects the "old" access token as
// expired (with the hint header) and accepts "new", plus a refresh endpoint that
// rotates "old-refresh" into a fresh pair.
func refreshServer(t *testing.T, refreshHits *int) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/api/unstable/protected", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "Bearer new" {
			_, _ = w.Write([]byte(`{"ok":true}`))
			return
		}
		w.Header().Set("X-Access-Token-Expired", "1")
		w.WriteHeader(http.StatusUnauthorized)
	})
	mux.HandleFunc("/api/unstable/auth/refresh", func(w http.ResponseWriter, r *http.Request) {
		*refreshHits++
		var body struct {
			RefreshToken string `json:"refresh_token"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body.RefreshToken != "old-refresh" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"token": "new", "refresh_token": "new-refresh", "login": "u", "id": 1,
		})
	})
	return httptest.NewServer(mux)
}

func protectedOp(base string) func(context.Context, string) error {
	return func(ctx context.Context, token string) error {
		if token == "new" {
			return nil
		}
		return errors.WrapErr(ErrAccessTokenExpired, "protected")
	}
}

// An expired access token is transparently refreshed, the rotated pair persisted,
// and the call retried once and succeeds.
func TestWithRefreshRecoversFromExpiredAccess(t *testing.T) {
	var hits int
	srv := refreshServer(t, &hits)
	defer srv.Close()

	var savedAccess, savedRefresh string
	persist := func(access, refresh string) error {
		savedAccess, savedRefresh = access, refresh
		return nil
	}
	if err := WithRefresh(context.Background(), srv.URL, "old", "old-refresh", persist, protectedOp(srv.URL)); err != nil {
		t.Fatalf("WithRefresh: %v", err)
	}
	if hits != 1 {
		t.Fatalf("refresh endpoint hit %d times, want 1", hits)
	}
	if savedAccess != "new" || savedRefresh != "new-refresh" {
		t.Fatalf("persisted %q/%q, want new/new-refresh", savedAccess, savedRefresh)
	}
}

// With no refresh token stored, the expired-access error surfaces unchanged (the
// caller prompts for a re-login) and no refresh is attempted.
func TestWithRefreshWithoutRefreshTokenSurfacesError(t *testing.T) {
	var hits int
	srv := refreshServer(t, &hits)
	defer srv.Close()

	err := WithRefresh(context.Background(), srv.URL, "old", "", nil, protectedOp(srv.URL))
	if !errors.Is(err, ErrAccessTokenExpired) {
		t.Fatalf("err = %v, want ErrAccessTokenExpired", err)
	}
	if hits != 0 {
		t.Fatalf("refresh endpoint hit %d times, want 0", hits)
	}
}

// A call that succeeds on the first try never refreshes.
func TestWithRefreshNoOpOnSuccess(t *testing.T) {
	var hits int
	srv := refreshServer(t, &hits)
	defer srv.Close()

	// "new" is already accepted by the protected endpoint.
	if err := WithRefresh(context.Background(), srv.URL, "new", "old-refresh", nil, protectedOp(srv.URL)); err != nil {
		t.Fatalf("WithRefresh: %v", err)
	}
	if hits != 0 {
		t.Fatalf("refresh endpoint hit %d times, want 0", hits)
	}
}
