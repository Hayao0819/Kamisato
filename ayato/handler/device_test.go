package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/Hayao0819/Kamisato/ayato/auth"
	"github.com/Hayao0819/Kamisato/ayato/repository"
	"github.com/Hayao0819/Kamisato/ayato/repository/kv/badgerkv"
	"github.com/Hayao0819/Kamisato/ayato/service"
	"github.com/Hayao0819/Kamisato/internal/conf"
)

// deviceHandler builds a handler with a real kv-backed device store, GitHub OAuth
// configured, and adminID seeded into the allowlist.
func deviceHandler(t *testing.T, adminID int64) (*AuthHandler, repository.DeviceRepository, *auth.Signer) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	store, err := badgerkv.New(t.TempDir())
	if err != nil {
		t.Fatalf("badgerkv.New: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	cfg := &conf.AyatoConfig{}
	cfg.Auth.GitHub.ClientID = "cid"
	cfg.Auth.GitHub.ClientSecret = "secret"
	cfg.Auth.PublicOrigin = "https://repo.example.com"

	authRepo := repository.NewAuthRepository(store)
	if err := authRepo.AddAdmin(adminID, "alice"); err != nil {
		t.Fatalf("AddAdmin: %v", err)
	}
	dev := repository.NewDeviceRepository(store)
	svc := service.New(nil, nil, authRepo, nil, cfg)
	signer, err := auth.NewSigner([]string{testSecret})
	if err != nil {
		t.Fatalf("NewSigner: %v", err)
	}
	h := NewAuthHandler(svc, svc, cfg).WithSigner(signer).WithDeviceStore(dev)
	return h, dev, signer
}

func pollDevice(t *testing.T, h *AuthHandler, deviceCode string) *httptest.ResponseRecorder {
	t.Helper()
	return postJSON(
		t,
		"/token",
		`{"device_code":"`+deviceCode+`"}`,
		h.DeviceTokenHandler,
	)
}

func deviceErr(t *testing.T, w *httptest.ResponseRecorder) string {
	t.Helper()
	var body struct {
		Error string `json:"error"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &body)
	return body.Error
}

// The full issuance path: a fresh code polls pending, an approval attaches the
// identity, the next poll mints a CLI token, and the code is then spent.
func TestDeviceTokenPendingApprovedThenSpent(t *testing.T) {
	h, dev, signer := deviceHandler(t, 42)
	if err := dev.CreateDevice("dc-1", "BCDF-GHJK", deviceCodeTTL); err != nil {
		t.Fatalf("CreateDevice: %v", err)
	}

	if w := pollDevice(t, h, "dc-1"); w.Code != http.StatusBadRequest || deviceErr(t, w) != "authorization_pending" {
		t.Fatalf("pending poll = %d %q, want 400 authorization_pending", w.Code, deviceErr(t, w))
	}

	if ok, err := dev.ApproveDevice("BCDF-GHJK", 42, "alice"); err != nil || !ok {
		t.Fatalf("ApproveDevice ok=%v err=%v", ok, err)
	}

	w := pollDevice(t, h, "dc-1")
	if w.Code != http.StatusOK {
		t.Fatalf("approved poll = %d, want 200 (body %s)", w.Code, w.Body.String())
	}
	var resp struct {
		Token string `json:"token"`
		Login string `json:"login"`
		ID    int64  `json:"id"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal token response: %v", err)
	}
	if resp.Login != "alice" || resp.ID != 42 {
		t.Fatalf("response login/id = %q/%d, want alice/42", resp.Login, resp.ID)
	}
	if _, err := signer.VerifyTyp(resp.Token, auth.TypCLI); err != nil {
		t.Fatalf("issued token must verify as a CLI token: %v", err)
	}

	// The device_code is consumed on success, so a replayed poll reads as expired.
	if w := pollDevice(t, h, "dc-1"); w.Code != http.StatusBadRequest || deviceErr(t, w) != "expired_token" {
		t.Fatalf("replayed poll = %d %q, want 400 expired_token", w.Code, deviceErr(t, w))
	}
}

// A denied authorization stops the client immediately with access_denied.
func TestDeviceTokenDenied(t *testing.T) {
	h, dev, _ := deviceHandler(t, 42)
	if err := dev.CreateDevice("dc-2", "MNPQ-RSTV", deviceCodeTTL); err != nil {
		t.Fatalf("CreateDevice: %v", err)
	}
	if ok, err := dev.DenyDevice("MNPQ-RSTV"); err != nil || !ok {
		t.Fatalf("DenyDevice ok=%v err=%v", ok, err)
	}
	if w := pollDevice(t, h, "dc-2"); w.Code != http.StatusBadRequest || deviceErr(t, w) != "access_denied" {
		t.Fatalf("denied poll = %d %q, want 400 access_denied", w.Code, deviceErr(t, w))
	}
}

// An unknown (or expired-and-evicted) device_code reads as expired_token.
func TestDeviceTokenExpired(t *testing.T) {
	h, _, _ := deviceHandler(t, 42)
	if w := pollDevice(t, h, "does-not-exist"); w.Code != http.StatusBadRequest || deviceErr(t, w) != "expired_token" {
		t.Fatalf("unknown poll = %d %q, want 400 expired_token", w.Code, deviceErr(t, w))
	}
}

// DeviceCodeHandler stores a pending record and advertises the verification URIs.
func TestDeviceCodeHandlerIssuesPending(t *testing.T) {
	h, _, _ := deviceHandler(t, 42)
	r := gin.New()
	r.POST("/code", h.DeviceCodeHandler)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/code", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("device/code = %d, want 200 (body %s)", w.Code, w.Body.String())
	}
	var resp struct {
		DeviceCode       string `json:"device_code"`
		UserCode         string `json:"user_code"`
		VerificationURI  string `json:"verification_uri"`
		VerificationComp string `json:"verification_uri_complete"`
		ExpiresIn        int    `json:"expires_in"`
		Interval         int    `json:"interval"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.DeviceCode == "" || resp.UserCode == "" || resp.ExpiresIn <= 0 || resp.Interval <= 0 {
		t.Fatalf("incomplete device code response: %+v", resp)
	}
	if !strings.Contains(resp.VerificationComp, url.QueryEscape(resp.UserCode)) {
		t.Fatalf("verification_uri_complete %q must embed the user_code", resp.VerificationComp)
	}
	// The freshly issued code must poll as pending.
	if w := pollDevice(t, h, resp.DeviceCode); deviceErr(t, w) != "authorization_pending" {
		t.Fatalf("fresh code poll = %q, want authorization_pending", deviceErr(t, w))
	}
}

// With the poll guard wired, a second poll inside the interval is answered
// slow_down.
func TestDeviceTokenSlowDown(t *testing.T) {
	h, dev, _ := deviceHandler(t, 42)
	store, err := badgerkv.New(t.TempDir())
	if err != nil {
		t.Fatalf("badgerkv.New: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	h.WithReplayGuard(repository.NewReplayGuard(store))

	if err := dev.CreateDevice("dc-fast", "FAST-POLL", deviceCodeTTL); err != nil {
		t.Fatalf("CreateDevice: %v", err)
	}
	if w := pollDevice(t, h, "dc-fast"); deviceErr(t, w) != "authorization_pending" {
		t.Fatalf("first poll = %q, want authorization_pending", deviceErr(t, w))
	}
	if w := pollDevice(t, h, "dc-fast"); deviceErr(t, w) != "slow_down" {
		t.Fatalf("second poll = %q, want slow_down", deviceErr(t, w))
	}
}
