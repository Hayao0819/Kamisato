package handler

import (
	"log/slog"

	"github.com/Hayao0819/Kamisato/ayato/auth"
	"github.com/Hayao0819/Kamisato/ayato/bugreport"
	"github.com/Hayao0819/Kamisato/ayato/recaptcha"
	"github.com/Hayao0819/Kamisato/ayato/service"
	"github.com/Hayao0819/Kamisato/internal/conf"
)

type Handler struct {
	cfg       *conf.AyatoConfig
	s         service.Servicer
	signer    *auth.Signer       // nil when auth is not wired (tests)
	reporter  bugreport.Reporter // nil when bug reporting is not configured
	recaptcha recaptcha.Verifier // nil when reCAPTCHA is not configured
}

func New(service service.Servicer, cfg *conf.AyatoConfig) *Handler {
	h := &Handler{
		s:   service,
		cfg: cfg,
	}
	if cfg != nil {
		// A bad bug-report config disables the feature rather than failing startup.
		reporter, err := bugreport.New(cfg.BugReport.Type, cfg.BugReport.GitHub.Repo, cfg.BugReport.GitHub.Token)
		if err != nil {
			slog.Error("bug reporting disabled: invalid config", "error", err)
		}
		h.reporter = reporter
		h.recaptcha = recaptcha.New(cfg.Recaptcha.Secret)
	}
	return h
}

// WithAuth attaches the stateless signer; set at startup, tests omit it (signer stays nil).
func (h *Handler) WithAuth(signer *auth.Signer) *Handler {
	h.signer = signer
	return h
}
