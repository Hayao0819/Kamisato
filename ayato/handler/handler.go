package handler

import (
	"log/slog"
	"time"

	"github.com/Hayao0819/Kamisato/ayato/auth"
	"github.com/Hayao0819/Kamisato/ayato/domain"
	"github.com/Hayao0819/Kamisato/ayato/handler/bugreport"
	"github.com/Hayao0819/Kamisato/ayato/handler/recaptcha"
	"github.com/Hayao0819/Kamisato/ayato/service"
	"github.com/Hayao0819/Kamisato/internal/conf"
)

type Handler struct {
	cfg       *conf.AyatoConfig
	catalog   *domain.RepositoryCatalog
	s         service.Servicer
	signer    *auth.Signer       // nil when auth is not wired (tests)
	reporter  bugreport.Reporter // nil when bug reporting is not configured
	recaptcha recaptcha.Verifier // nil when reCAPTCHA is not configured
	replay    replayGuard        // nil when the one-time code guard is not wired
	logTokens logTokenMinter     // nil when SSE log tokens are not wired
	device    deviceStore        // nil when the device-authorization flow is not wired
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

// replayGuard records a one-time PKCE code id at redemption so a replayed code is
// rejected.
type replayGuard interface {
	Consume(id string, ttl time.Duration) (firstUse bool, err error)
}

// logTokenMinter issues one-time SSE log-stream tokens bound to a job id.
type logTokenMinter interface {
	Mint(jobID string, ttl time.Duration) (string, error)
}

func New(service service.Servicer, cfg *conf.AyatoConfig) *Handler {
	catalog, _ := domain.NewRepositoryCatalog(nil, nil)
	h := &Handler{
		s:       service,
		cfg:     cfg,
		catalog: catalog,
	}
	if cfg != nil {
		if configuredCatalog, err := cfg.RepositoryCatalog(); err == nil {
			h.catalog = configuredCatalog
		} else {
			slog.Error("invalid repository catalog", "error", err)
		}
		// A bad bug-report config disables the feature rather than failing startup.
		reporter, err := bugreport.New(bugReportConfig(cfg.BugReport))
		if err != nil {
			slog.Error("bug reporting disabled: invalid config", "error", err)
		}
		h.reporter = reporter
		h.recaptcha = recaptcha.New(cfg.Recaptcha.Provider, cfg.Recaptcha.Secret)
	}
	return h
}

// bugReportConfig maps the on-disk config into the bugreport package's own Config
// (kept separate so bugreport never imports internal/conf).
func bugReportConfig(c conf.BugReportConfig) bugreport.Config {
	return bugreport.Config{
		Backends: c.Backends,
		GitHub:   bugreport.GitHubConfig{Repo: c.GitHub.Repo, Token: c.GitHub.Token},
		SMTP: bugreport.SMTPConfig{
			Host:         c.SMTP.Host,
			Port:         c.SMTP.Port,
			Username:     c.SMTP.Username,
			Password:     c.SMTP.Password,
			From:         c.SMTP.From,
			To:           c.SMTP.To,
			ToMaintainer: c.SMTP.ToMaintainer,
		},
		Webhook: bugreport.WebhookConfig{URL: c.Webhook.URL},
	}
}

// WithAuth attaches the stateless signer; set at startup, tests omit it (signer stays nil).
func (h *Handler) WithAuth(signer *auth.Signer) *Handler {
	h.signer = signer
	return h
}

// WithReplayGuard attaches the kv-backed one-time code guard. Unwired (nil) means
// codes are replay-limited only by their TTL.
func (h *Handler) WithReplayGuard(g replayGuard) *Handler {
	h.replay = g
	return h
}

// WithLogTokens attaches the one-time SSE log-token minter. Unwired (nil) means the
// mint endpoint reports unavailable and only bearer/session access to /logs works.
func (h *Handler) WithLogTokens(m logTokenMinter) *Handler {
	h.logTokens = m
	return h
}

// WithDeviceStore attaches the kv-backed device-authorization store. Unwired (nil)
// makes the device endpoints report the flow unavailable.
func (h *Handler) WithDeviceStore(d deviceStore) *Handler {
	h.device = d
	return h
}
