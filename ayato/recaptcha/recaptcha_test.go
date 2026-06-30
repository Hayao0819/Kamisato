package recaptcha

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewDisabledWhenNoSecret(t *testing.T) {
	if New("") != nil {
		t.Fatal("empty secret must yield a nil (disabled) verifier")
	}
	if New("s") == nil {
		t.Fatal("a non-empty secret must yield a verifier")
	}
}

func TestVerify(t *testing.T) {
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
	v := &googleVerifier{secret: "S", endpoint: srv.URL, client: srv.Client()}

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
}
