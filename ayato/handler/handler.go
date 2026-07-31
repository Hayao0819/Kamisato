package handler

import (
	"time"

	"github.com/Hayao0819/Kamisato/ayato/auth"
	"github.com/Hayao0819/Kamisato/ayato/domain"
	"github.com/Hayao0819/Kamisato/ayato/handler/bugreport"
	"github.com/Hayao0819/Kamisato/ayato/handler/recaptcha"
	"github.com/Hayao0819/Kamisato/ayato/service"
)

type Settings struct {
	Catalog                  *domain.RepositoryCatalog
	DisableRedirectDownloads bool
	MaxSize                  int
	MaxBatchPackages         int
	MaxBatchBytes            int64
	RequireSign              bool
	Miko                     MikoSettings
	Auth                     AuthSettings
	Mirror                   MirrorSettings
	Recaptcha                RecaptchaSettings
	BugReport                bugreport.Config
}

type MikoSettings struct {
	URL    string
	APIKey string
}

type AuthSettings struct {
	GitHubClientID     string
	GitHubClientSecret string
	PublicOrigin       string
	SelfOrigin         string
	CookieName         string
	AccessTokenTTL     time.Duration
	RefreshTokenTTL    time.Duration
}

type MirrorSettings struct {
	SelfURL      string
	ServerPath   string
	UseRepoVar   bool
	AllCommented bool
}

type RecaptchaSettings struct {
	Provider string
	SiteKey  string
	Secret   string
}

func (settings Settings) normalized() Settings {
	if settings.Catalog == nil {
		settings.Catalog, _ = domain.NewRepositoryCatalog(nil, nil)
	}
	if settings.Auth.CookieName == "" {
		settings.Auth.CookieName = "__Host-ayato_session"
	}
	if settings.Auth.AccessTokenTTL <= 0 {
		settings.Auth.AccessTokenTTL = time.Hour
	}
	if settings.Auth.RefreshTokenTTL <= 0 {
		settings.Auth.RefreshTokenTTL = 30 * 24 * time.Hour
	}
	if settings.Mirror.ServerPath == "" {
		settings.Mirror.ServerPath = "/repo"
	}
	return settings
}

// Set is the HTTP composition root. It contains feature-scoped handlers rather
// than implementing every endpoint on one service-locator-style type.
type Set struct {
	System       *SystemHandler
	Repositories *RepositoryHandler
	Publications *PublicationHandler
	Auth         *AuthHandler
	Admins       *AdminHandler
	Signers      *SignerHandler
	BugReports   *BugReportHandler
	Miko         *MikoHandler
}

type SystemHandler struct {
	settings         Settings
	bugReportEnabled bool
	oauthEnabled     func() bool
}

type RepositoryHandler struct {
	settings Settings
	catalog  *domain.RepositoryCatalog
	reader   service.RepoReader
}

type PublicationHandler struct {
	settings Settings
	uploader service.Uploader
	promoter service.Promoter
	syncer   service.Syncer
}

type AuthHandler struct {
	settings Settings
	admins   service.AdminService
	revoker  service.Revoker
	signer   *auth.Signer
	replay   replayGuard
	device   deviceStore
}

type AdminHandler struct {
	admins service.AdminService
}

type SignerHandler struct {
	signers service.SignerRegistry
}

type BugReportHandler struct {
	reader    service.RepoReader
	reporter  bugreport.Reporter
	recaptcha recaptcha.Verifier
}

type MikoHandler struct {
	settings  Settings
	logTokens logTokenStore
}

// deviceStore is the RFC 8628 device-authorization rendezvous; a narrow local
// interface keeps the handler off the repository package.
type deviceStore interface {
	CreateDevice(deviceCode, userCode string, ttl time.Duration) error
	LookupByUserCode(userCode string) (status string, ok bool, err error)
	ApproveDevice(userCode string, githubID int64, login string) (ok bool, err error)
	DenyDevice(userCode string) (ok bool, err error)
	PollDevice(deviceCode string) (status string, githubID int64, login string, ok bool, err error)
	ConsumeDevice(deviceCode string) (consumed bool, err error)
}

type replayGuard interface {
	Consume(id string, ttl time.Duration) (firstUse bool, err error)
}

type logTokenStore interface {
	StoreLogToken(token, jobID string, ttl time.Duration) error
}
