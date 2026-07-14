package recaptcha

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewDisabledWhenNoSecret(t *testing.T) {
	for _, provider := range []string{"", "recaptcha", "turnstile"} {
		if New(provider, "") != nil {
			t.Errorf("provider %q: empty secret must yield a nil (disabled) verifier", provider)
		}
		if New(provider, "s") == nil {
			t.Errorf("provider %q: a non-empty secret must yield a verifier", provider)
		}
	}
}

func TestVerify(t *testing.T) {
	for _, tc := range []struct {
		provider string
		endpoint string
	}{
		{"recaptcha", recaptchaVerifyURL},
		{"", recaptchaVerifyURL},
		{"turnstile", turnstileVerifyURL},
	} {
		t.Run(tc.provider, func(t *testing.T) {
			v, ok := New(tc.provider, "S").(*verifier)
			if !ok {
				t.Fatalf("New(%q) did not return *verifier", tc.provider)
			}
			if v.endpoint != tc.endpoint {
				t.Fatalf("provider %q hits endpoint %q, want %q", tc.provider, v.endpoint, tc.endpoint)
			}

			var gotSecret, gotResponse string
			success := true
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_ = r.ParseForm()
				gotSecret = r.FormValue("secret")
				gotResponse = r.FormValue("response")
				w.Header().Set("Content-Type", "application/json")
				if success {
					_, _ = w.Write([]byte(`{"success":true}`))
				} else {
					_, _ = w.Write([]byte(`{"success":false,"error-codes":["invalid-input-response"]}`))
				}
			}))
			defer srv.Close()
			v.endpoint = srv.URL
			v.client = srv.Client()

			if err := v.Verify(context.Background(), "", ""); err == nil {
				t.Error("a missing token must error without calling the endpoint")
			}

			success = false
			if err := v.Verify(context.Background(), "tok", "1.2.3.4"); err == nil {
				t.Error("an unsuccessful verification must error")
			}

			success = true
			if err := v.Verify(context.Background(), "tok", ""); err != nil {
				t.Fatalf("successful verification: %v", err)
			}
			if gotSecret != "S" || gotResponse != "tok" {
				t.Errorf("siteverify got secret=%q response=%q, want S/tok", gotSecret, gotResponse)
			}
		})
	}
}
