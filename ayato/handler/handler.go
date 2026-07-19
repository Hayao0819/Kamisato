package handler

import (
	"time"

	"github.com/Hayao0819/Kamisato/ayato/auth"
	"github.com/Hayao0819/Kamisato/ayato/domain"
	"github.com/Hayao0819/Kamisato/ayato/handler/bugreport"
	"github.com/Hayao0819/Kamisato/ayato/handler/recaptcha"
	"github.com/Hayao0819/Kamisato/ayato/service"
	"github.com/Hayao0819/Kamisato/internal/conf"
)

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
	cfg              *conf.AyatoConfig
	bugReportEnabled bool
	oauthEnabled     func() bool
}

type RepositoryHandler struct {
	cfg     *conf.AyatoConfig
	catalog *domain.RepositoryCatalog
	reader  service.RepoReader
}

type PublicationHandler struct {
	cfg      *conf.AyatoConfig
	uploader service.Uploader
	promoter service.Promoter
	syncer   service.Syncer
}

type AuthHandler struct {
	cfg     *conf.AyatoConfig
	admins  service.AdminService
	revoker service.Revoker
	signer  *auth.Signer
	replay  replayGuard
	device  deviceStore
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
	cfg       *conf.AyatoConfig
	logTokens logTokenMinter
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

type logTokenMinter interface {
	Mint(jobID string, ttl time.Duration) (string, error)
}
