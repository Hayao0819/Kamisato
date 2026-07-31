package handler

import (
	"log/slog"

	"github.com/Hayao0819/Kamisato/ayato/auth"
	"github.com/Hayao0819/Kamisato/ayato/handler/bugreport"
	"github.com/Hayao0819/Kamisato/ayato/handler/recaptcha"
	"github.com/Hayao0819/Kamisato/ayato/service"
)

// New composes independently constructible feature handlers from the production
// service. Only this boundary depends on the broad Servicer interface.
func New(s service.Servicer, settings Settings) *Set {
	settings = settings.normalized()
	authHandler := NewAuthHandler(s, s, settings)
	bugReports := NewBugReportHandler(s, settings)
	return &Set{
		System:       NewSystemHandler(settings, bugReports.reporter != nil, authHandler.oauthConfigured),
		Repositories: NewRepositoryHandler(s, settings),
		Publications: NewPublicationHandler(s, s, s, settings),
		Auth:         authHandler,
		Admins:       NewAdminHandler(s),
		Signers:      NewSignerHandler(s),
		BugReports:   bugReports,
		Miko:         NewMikoHandler(settings),
	}
}

func NewSystemHandler(
	settings Settings,
	bugReportEnabled bool,
	oauthEnabled func() bool,
) *SystemHandler {
	return &SystemHandler{settings: settings.normalized(), bugReportEnabled: bugReportEnabled, oauthEnabled: oauthEnabled}
}

func NewRepositoryHandler(reader service.RepoReader, settings Settings) *RepositoryHandler {
	settings = settings.normalized()
	return &RepositoryHandler{settings: settings, catalog: settings.Catalog, reader: reader}
}

func NewPublicationHandler(
	uploader service.Uploader,
	promoter service.Promoter,
	syncer service.Syncer,
	settings Settings,
) *PublicationHandler {
	return &PublicationHandler{settings: settings.normalized(), uploader: uploader, promoter: promoter, syncer: syncer}
}

func NewAuthHandler(
	admins service.AdminService,
	revoker service.Revoker,
	settings Settings,
) *AuthHandler {
	return &AuthHandler{settings: settings.normalized(), admins: admins, revoker: revoker}
}

func NewAdminHandler(admins service.AdminService) *AdminHandler {
	return &AdminHandler{admins: admins}
}

func NewSignerHandler(signers service.SignerRegistry) *SignerHandler {
	return &SignerHandler{signers: signers}
}

func NewBugReportHandler(reader service.RepoReader, settings Settings) *BugReportHandler {
	h := &BugReportHandler{reader: reader}
	reporter, err := bugreport.New(settings.BugReport)
	if err != nil {
		slog.Error("bug reporting disabled: invalid config", "error", err)
	}
	h.reporter = reporter
	h.recaptcha = recaptcha.New(settings.Recaptcha.Provider, settings.Recaptcha.Secret)
	return h
}

func NewMikoHandler(settings Settings) *MikoHandler {
	return &MikoHandler{settings: settings.normalized()}
}

func (s *Set) WithAuth(signer *auth.Signer) *Set {
	s.Auth.WithSigner(signer)
	return s
}

func (s *Set) WithReplayGuard(guard replayGuard) *Set {
	s.Auth.WithReplayGuard(guard)
	return s
}

func (s *Set) WithLogTokens(tokens logTokenStore) *Set {
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

func (h *MikoHandler) WithLogTokens(tokens logTokenStore) *MikoHandler {
	h.logTokens = tokens
	return h
}
