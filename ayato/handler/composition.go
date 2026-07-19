package handler

import (
	"log/slog"

	"github.com/Hayao0819/Kamisato/ayato/auth"
	"github.com/Hayao0819/Kamisato/ayato/domain"
	"github.com/Hayao0819/Kamisato/ayato/handler/bugreport"
	"github.com/Hayao0819/Kamisato/ayato/handler/recaptcha"
	"github.com/Hayao0819/Kamisato/ayato/service"
	"github.com/Hayao0819/Kamisato/internal/conf"
)

// New composes independently constructible feature handlers from the production
// service. Only this boundary depends on the broad Servicer interface.
func New(s service.Servicer, cfg *conf.AyatoConfig) *Set {
	authHandler := NewAuthHandler(s, s, cfg)
	bugReports := NewBugReportHandler(s, cfg)
	return &Set{
		System:       NewSystemHandler(cfg, bugReports.reporter != nil, authHandler.oauthConfigured),
		Repositories: NewRepositoryHandler(s, cfg),
		Publications: NewPublicationHandler(s, s, s, cfg),
		Auth:         authHandler,
		Admins:       NewAdminHandler(s),
		Signers:      NewSignerHandler(s),
		BugReports:   bugReports,
		Miko:         NewMikoHandler(cfg),
	}
}

func NewSystemHandler(
	cfg *conf.AyatoConfig,
	bugReportEnabled bool,
	oauthEnabled func() bool,
) *SystemHandler {
	return &SystemHandler{cfg: cfg, bugReportEnabled: bugReportEnabled, oauthEnabled: oauthEnabled}
}

func NewRepositoryHandler(reader service.RepoReader, cfg *conf.AyatoConfig) *RepositoryHandler {
	catalog, _ := domain.NewRepositoryCatalog(nil, nil)
	if cfg != nil {
		if configured, err := cfg.RepositoryCatalog(); err == nil {
			catalog = configured
		} else {
			slog.Error("invalid repository catalog", "error", err)
		}
	}
	return &RepositoryHandler{cfg: cfg, catalog: catalog, reader: reader}
}

func NewPublicationHandler(
	uploader service.Uploader,
	promoter service.Promoter,
	syncer service.Syncer,
	cfg *conf.AyatoConfig,
) *PublicationHandler {
	return &PublicationHandler{cfg: cfg, uploader: uploader, promoter: promoter, syncer: syncer}
}

func NewAuthHandler(
	admins service.AdminService,
	revoker service.Revoker,
	cfg *conf.AyatoConfig,
) *AuthHandler {
	return &AuthHandler{cfg: cfg, admins: admins, revoker: revoker}
}

func NewAdminHandler(admins service.AdminService) *AdminHandler {
	return &AdminHandler{admins: admins}
}

func NewSignerHandler(signers service.SignerRegistry) *SignerHandler {
	return &SignerHandler{signers: signers}
}

func NewBugReportHandler(reader service.RepoReader, cfg *conf.AyatoConfig) *BugReportHandler {
	h := &BugReportHandler{reader: reader}
	if cfg == nil {
		return h
	}
	reporter, err := bugreport.New(bugReportConfig(cfg.BugReport))
	if err != nil {
		slog.Error("bug reporting disabled: invalid config", "error", err)
	}
	h.reporter = reporter
	h.recaptcha = recaptcha.New(cfg.Recaptcha.Provider, cfg.Recaptcha.Secret)
	return h
}

func NewMikoHandler(cfg *conf.AyatoConfig) *MikoHandler {
	return &MikoHandler{cfg: cfg}
}

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

func (s *Set) WithAuth(signer *auth.Signer) *Set {
	s.Auth.WithSigner(signer)
	return s
}

func (s *Set) WithReplayGuard(guard replayGuard) *Set {
	s.Auth.WithReplayGuard(guard)
	return s
}

func (s *Set) WithLogTokens(tokens logTokenMinter) *Set {
	s.Miko.WithLogTokens(tokens)
	return s
}

func (s *Set) WithDeviceStore(store deviceStore) *Set {
	s.Auth.WithDeviceStore(store)
	return s
}

func (h *AuthHandler) WithSigner(signer *auth.Signer) *AuthHandler {
	h.signer = signer
	return h
}

func (h *AuthHandler) WithReplayGuard(guard replayGuard) *AuthHandler {
	h.replay = guard
	return h
}

func (h *AuthHandler) WithDeviceStore(store deviceStore) *AuthHandler {
	h.device = store
	return h
}

func (h *MikoHandler) WithLogTokens(tokens logTokenMinter) *MikoHandler {
	h.logTokens = tokens
	return h
}
