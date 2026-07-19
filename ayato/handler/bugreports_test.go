package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Hayao0819/Kamisato/internal/errors"

	"github.com/gin-gonic/gin"
	"go.uber.org/mock/gomock"

	"github.com/Hayao0819/Kamisato/ayato/domain"
	"github.com/Hayao0819/Kamisato/ayato/handler/bugreport"
	"github.com/Hayao0819/Kamisato/ayato/test/mocks"
	"github.com/Hayao0819/Kamisato/pkg/raiou"
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

func TestSubmitBugReportResolvesMaintainer(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockSvc := mocks.NewMockServicer(ctrl)
	mockSvc.EXPECT().PkgDetail("core", "x86_64", "foo").
		Return(&domain.PacmanPackage{PKGINFO: raiou.PKGINFO{Packager: "Maintainer Name <maint@example.com>"}}, nil)

	fr := &fakeReporter{}
	h := &Handler{s: mockSvc, reporter: fr}
	body := `{"pkgname":"foo","repo":"core","arch":"x86_64","description":"boom","email":"reporter@example.com"}`
	if w := postBug(h, body); w.Code != http.StatusCreated {
		t.Fatalf("status %d, want 201", w.Code)
	}
	if fr.last.MaintainerEmail != "maint@example.com" {
		t.Errorf("maintainer = %q, want maint@example.com (parsed from Packager)", fr.last.MaintainerEmail)
	}
	// The client-supplied email must never become the maintainer recipient.
	if fr.last.Email != "reporter@example.com" {
		t.Errorf("reporter email = %q", fr.last.Email)
	}
	if fr.last.MaintainerEmail == fr.last.Email {
		t.Errorf("client email leaked into maintainer routing")
	}
}

func TestSubmitBugReportMaintainerLookupFailureIsNonFatal(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockSvc := mocks.NewMockServicer(ctrl)
	mockSvc.EXPECT().PkgDetail("core", "x86_64", "foo").Return(nil, errors.New("not found"))

	fr := &fakeReporter{}
	h := &Handler{s: mockSvc, reporter: fr}
	body := `{"pkgname":"foo","repo":"core","arch":"x86_64","description":"boom"}`
	if w := postBug(h, body); w.Code != http.StatusCreated {
		t.Fatalf("a failed maintainer lookup must not fail the report: status %d", w.Code)
	}
	if fr.last.MaintainerEmail != "" {
		t.Errorf("maintainer = %q, want empty when lookup fails", fr.last.MaintainerEmail)
	}
}

func TestParsePackagerEmail(t *testing.T) {
	cases := map[string]string{
		"Real Name <dev@example.com>": "dev@example.com",
		"dev@example.com":             "dev@example.com",
		"":                            "",
		"Nobody":                      "",
	}
	for in, want := range cases {
		if got := parsePackagerEmail(in); got != want {
			t.Errorf("parsePackagerEmail(%q) = %q, want %q", in, got, want)
		}
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
