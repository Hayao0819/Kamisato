package middleware

import (
	"context"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/Hayao0819/Kamisato/ayato/auth"
	"github.com/Hayao0819/Kamisato/internal/conf"
)

func TestRequireServiceScope(t *testing.T) {
	gin.SetMode(gin.TestMode)
	authorizer, err := auth.NewCIAuthorizer(context.Background(), conf.CIAuthConfig{APIKeys: []conf.CIAPIKey{
		{Name: "signer", Key: "right-key", Scopes: []string{"signer:register"}},
	}})
	if err != nil {
		t.Fatal(err)
	}
	m := New(&conf.AyatoConfig{}).WithCIAuth(authorizer)
	router := gin.New()
	router.POST("/signers", m.RequireServiceScope("signer:register"), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	for _, tc := range []struct {
		name, key string
		want      int
	}{
		{name: "scoped key", key: "right-key", want: http.StatusOK},
		{name: "invalid key cannot fall through", key: "wrong-key", want: http.StatusForbidden},
		{name: "missing key requires admin", want: http.StatusServiceUnavailable},
	} {
		t.Run(tc.name, func(t *testing.T) {
			response := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodPost, "/signers", nil)
			if tc.key != "" {
				request.Header.Set("X-API-Key", tc.key)
			}
			router.ServeHTTP(response, request)
			if response.Code != tc.want {
				t.Fatalf("status = %d, want %d", response.Code, tc.want)
			}
		})
	}
}

func TestSignerRegistrationLegacyBasicRequiresExplicitFlagAndNeverDowngradesAPIKey(t *testing.T) {
	gin.SetMode(gin.TestMode)
	const secret = "0123456789abcdef0123456789abcdef"
	signer, err := auth.NewSigner([]string{secret})
	if err != nil {
		t.Fatal(err)
	}
	admin, err := signer.Sign(auth.Claims{Typ: auth.TypCLI, GitHubID: 42, Exp: time.Now().Add(time.Hour)})
	if err != nil {
		t.Fatal(err)
	}
	authorizer, err := auth.NewCIAuthorizer(context.Background(), conf.CIAuthConfig{APIKeys: []conf.CIAPIKey{
		{Name: "signer", Key: "service-key", Scopes: []string{"signer:register"}},
	}})
	if err != nil {
		t.Fatal(err)
	}
	cfg := &conf.AyatoConfig{}
	cfg.Auth.AllowLegacySignerBasic = true
	m := New(cfg).WithAuth(fakeChecker{allowed: map[int64]bool{42: true}}, signer).WithCIAuth(authorizer)
	router := gin.New()
	router.POST("/signers", m.RequireSignerRegistration(), func(c *gin.Context) { c.Status(http.StatusOK) })

	run := func(apiKey string, basic bool) int {
		request := httptest.NewRequest(http.MethodPost, "/signers", nil)
		if apiKey != "" {
			request.Header.Set("X-API-Key", apiKey)
		}
		if basic {
			request.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte("legacy:"+admin)))
		}
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)
		return response.Code
	}
	if got := run("", true); got != http.StatusOK {
		t.Fatalf("explicit legacy Basic status = %d", got)
	}
	if got := run("wrong", true); got != http.StatusForbidden {
		t.Fatalf("invalid API key downgraded to Basic: status = %d", got)
	}

	cfg.Auth.AllowLegacySignerBasic = false
	if got := run("", true); got == http.StatusOK {
		t.Fatal("legacy Basic succeeded after migration flag was disabled")
	}
}
