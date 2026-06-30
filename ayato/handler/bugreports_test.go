package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Hayao0819/Kamisato/ayato/bugreport"
	"github.com/Hayao0819/Kamisato/internal/conf"
	"github.com/gin-gonic/gin"
)

type fakeReporter struct {
	last bugreport.Report
	url  string
	err  error
}

func (f *fakeReporter) Report(_ context.Context, r bugreport.Report) (string, error) {
	f.last = r
	return f.url, f.err
}

type fakeVerifier struct {
	called bool
	err    error
}

func (f *fakeVerifier) Verify(_ context.Context, _, _ string) error {
	f.called = true
	return f.err
}

func TestFeaturesHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := &conf.AyatoConfig{}
	cfg.Miko.URL = "http://miko:8081"
	cfg.Recaptcha.SiteKey = "SITE"
	h := &Handler{cfg: cfg, reporter: &fakeReporter{}}

	r := gin.New()
	r.GET("/features", h.FeaturesHandler)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/features", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}
	var got struct {
		BugReport        bool   `json:"bug_report"`
		Miko             bool   `json:"miko"`
		GitHubLogin      bool   `json:"github_login"`
		RecaptchaSiteKey string `json:"recaptcha_site_key"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if !got.BugReport || !got.Miko || got.GitHubLogin || got.RecaptchaSiteKey != "SITE" {
		t.Errorf("features = %+v (want bug_report+miko on, github off, site SITE)", got)
	}
}

func postBug(h *Handler, body string) *httptest.ResponseRecorder {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/bug-reports", h.SubmitBugReportHandler)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/bug-reports", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	return w
}

func TestSubmitBugReport(t *testing.T) {
	if w := postBug(&Handler{}, `{"pkgname":"foo","description":"x"}`); w.Code != http.StatusServiceUnavailable {
		t.Errorf("disabled: status %d, want 503", w.Code)
	}
	if w := postBug(&Handler{reporter: &fakeReporter{}}, `{"pkgname":"foo"}`); w.Code != http.StatusBadRequest {
		t.Errorf("missing description: status %d, want 400", w.Code)
	}
	if w := postBug(&Handler{reporter: &fakeReporter{}}, `{"pkgname":"foo","description":"x","severity":"meh"}`); w.Code != http.StatusBadRequest {
		t.Errorf("bad severity: status %d, want 400", w.Code)
	}

	fr := &fakeReporter{url: "https://github.com/o/r/issues/1"}
	w := postBug(&Handler{reporter: fr}, `{"pkgname":"foo","pkgver":"1.0","description":"boom","severity":"high"}`)
	if w.Code != http.StatusCreated {
		t.Fatalf("success: status %d, want 201", w.Code)
	}
	if fr.last.Pkgname != "foo" || fr.last.Severity != "high" || fr.last.Description != "boom" {
		t.Errorf("reporter received %+v", fr.last)
	}
	if !strings.Contains(w.Body.String(), "issues/1") {
		t.Errorf("body %q missing the issue url", w.Body.String())
	}

	def := &fakeReporter{}
	postBug(&Handler{reporter: def}, `{"pkgname":"foo","description":"x"}`)
	if def.last.Severity != "medium" {
		t.Errorf("default severity = %q, want medium", def.last.Severity)
	}
}

func TestSubmitBugReportRecaptcha(t *testing.T) {
	// A configured verifier that rejects blocks the report (reporter not called).
	rep := &fakeReporter{}
	bad := &fakeVerifier{err: errors.New("nope")}
	if w := postBug(&Handler{reporter: rep, recaptcha: bad}, `{"pkgname":"foo","description":"x","recaptcha_token":"t"}`); w.Code != http.StatusBadRequest {
		t.Errorf("rejected captcha: status %d, want 400", w.Code)
	}
	if !bad.called || rep.last.Pkgname != "" {
		t.Errorf("captcha must be checked before forwarding; verified=%v forwarded=%q", bad.called, rep.last.Pkgname)
	}

	// A passing verifier lets the report through.
	rep2 := &fakeReporter{}
	ok := &fakeVerifier{}
	if w := postBug(&Handler{reporter: rep2, recaptcha: ok}, `{"pkgname":"foo","description":"x","recaptcha_token":"t"}`); w.Code != http.StatusCreated {
		t.Errorf("accepted captcha: status %d, want 201", w.Code)
	}
	if !ok.called || rep2.last.Pkgname != "foo" {
		t.Errorf("captcha pass must forward; verified=%v forwarded=%q", ok.called, rep2.last.Pkgname)
	}
}
