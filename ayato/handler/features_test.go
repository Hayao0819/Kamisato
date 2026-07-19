package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/Hayao0819/Kamisato/ayato/domain"
	"github.com/Hayao0819/Kamisato/internal/conf"
)

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
	var got domain.Features
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if !got.BugReport || !got.Miko || got.GitHubLogin ||
		got.RecaptchaSiteKey != "SITE" || len(got.PackageArchiveSuffixes) == 0 {
		t.Errorf("features = %+v (want bug_report+miko on, github off, site SITE, package suffixes)", got)
	}
}
