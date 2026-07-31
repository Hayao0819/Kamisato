package handler

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"go.uber.org/mock/gomock"

	"github.com/Hayao0819/Kamisato/ayato/blob"
	ayatoconfig "github.com/Hayao0819/Kamisato/ayato/config"
	"github.com/Hayao0819/Kamisato/ayato/domain"
	"github.com/Hayao0819/Kamisato/ayato/handler/bugreport"
	"github.com/Hayao0819/Kamisato/ayato/test/mocks"
)

const (
	testSecret  = "0123456789abcdef0123456789abcdef" // 32 bytes
	testAdminID = int64(42)
)

func setup(t *testing.T) (*gomock.Controller, *mocks.MockServicer, *Set) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	controller := gomock.NewController(t)
	service := mocks.NewMockServicer(controller)
	return controller, service, New(service, Settings{})
}

func testSettings(cfg *ayatoconfig.AyatoConfig) Settings {
	if cfg == nil {
		return Settings{}
	}
	catalog, _ := cfg.RepositoryCatalog()
	return Settings{
		Catalog:                  catalog,
		DisableRedirectDownloads: !cfg.RedirectDownloadsEnabled(),
		MaxSize:                  cfg.MaxSize,
		MaxBatchPackages:         cfg.MaxBatchPackages,
		MaxBatchBytes:            cfg.MaxBatchBytes,
		RequireSign:              cfg.RequireSign,
		Miko:                     MikoSettings{URL: cfg.Miko.URL, APIKey: cfg.Miko.APIKey},
		Auth: AuthSettings{
			GitHubClientID:     cfg.Auth.GitHub.ClientID,
			GitHubClientSecret: cfg.Auth.GitHub.ClientSecret,
			PublicOrigin:       cfg.Auth.PublicOrigin,
			SelfOrigin:         cfg.Auth.SelfOrigin,
			CookieName:         cfg.Auth.CookieName(),
			AccessTokenTTL:     cfg.Auth.AccessTokenTTL(),
			RefreshTokenTTL:    cfg.Auth.RefreshTokenTTL(),
		},
		Mirror: MirrorSettings{
			SelfURL:      cfg.Mirror.SelfURL,
			ServerPath:   cfg.Mirror.ServerPath(),
			UseRepoVar:   cfg.Mirror.UseRepoVar,
			AllCommented: cfg.Mirror.AllCommented,
		},
		Recaptcha: RecaptchaSettings{
			Provider: cfg.Recaptcha.Provider,
			SiteKey:  cfg.Recaptcha.SiteKey,
			Secret:   cfg.Recaptcha.Secret,
		},
		BugReport: bugreport.Config{
			Backends: cfg.BugReport.Backends,
			GitHub:   bugreport.GitHubConfig{Repo: cfg.BugReport.GitHub.Repo, Token: cfg.BugReport.GitHub.Token},
			SMTP: bugreport.SMTPConfig{
				Host:         cfg.BugReport.SMTP.Host,
				Port:         cfg.BugReport.SMTP.Port,
				Username:     cfg.BugReport.SMTP.Username,
				Password:     cfg.BugReport.SMTP.Password,
				From:         cfg.BugReport.SMTP.From,
				To:           cfg.BugReport.SMTP.To,
				ToMaintainer: cfg.BugReport.SMTP.ToMaintainer,
			},
			Webhook: bugreport.WebhookConfig{URL: cfg.BugReport.Webhook.URL},
		},
	}
}

func postJSON(
	t *testing.T,
	path, body string,
	handler gin.HandlerFunc,
) *httptest.ResponseRecorder {
	t.Helper()
	router := gin.New()
	router.POST(path, handler)
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(response, request)
	return response
}

func expectRepositoryFile(
	service *mocks.MockServicer,
	repo, arch, name, body string,
	meta domain.FileMeta,
	times int,
) {
	service.EXPECT().SignedURL(repo, arch, name).Return("", nil).Times(times)
	service.EXPECT().GetFileWithMeta(repo, arch, name).DoAndReturn(
		func(_, _, _ string) (blob.File, domain.FileMeta, error) {
			file := blob.NewFileStream(
				name,
				"application/octet-stream",
				bufferToReadSeekCloser(bytes.NewBufferString(body)),
			)
			return file, meta, nil
		},
	).Times(times)
}

var errTest = &testError{"boom"}

type testError struct{ msg string }

func (e *testError) Error() string { return e.msg }
