package handler

import (
	"log/slog"

	"github.com/Hayao0819/Kamisato/ayato/auth"
	"github.com/Hayao0819/Kamisato/ayato/bugreport"
	"github.com/Hayao0819/Kamisato/ayato/recaptcha"
	"github.com/Hayao0819/Kamisato/ayato/repository"
	"github.com/Hayao0819/Kamisato/ayato/service"
	"github.com/Hayao0819/Kamisato/internal/conf"
)

type Handler struct {
	cfg       *conf.AyatoConfig
	s         service.Servicer
	signer    *auth.Signer                  // nil when auth is not wired (tests)
	reporter  bugreport.Reporter            // nil when bug reporting is not configured
	recaptcha recaptcha.Verifier            // nil when reCAPTCHA is not configured
	denylist  repository.DenylistRepository // nil when per-token revocation is not wired
}

func New(service service.Servicer, cfg *conf.AyatoConfig) *Handler {
	h := &Handler{
		s:   service,
		cfg: cfg,
	}
	if cfg != nil {
		// A bad bug-report config disables the feature rather than failing startup.
		reporter, err := bugreport.New(bugReportConfig(cfg.BugReport))
		if err != nil {
			slog.Error("bug reporting disabled: invalid config", "error", err)
		}
		h.reporter = reporter
		h.recaptcha = recaptcha.New(cfg.Recaptcha.Secret)
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

// WithDenylist attaches the per-token revocation store used by the revoke
// endpoint and the MeHandler check.
func (h *Handler) WithDenylist(dl repository.DenylistRepository) *Handler {
	h.denylist = dl
	return h
}
