package handler

import (
	"github.com/Hayao0819/Kamisato/ayato/auth"
	"github.com/Hayao0819/Kamisato/ayato/service"
	"github.com/Hayao0819/Kamisato/internal/conf"
)

// Handler is a struct that handles API requests.
// Eventually planned to depend only on Service.
type Handler struct {
	cfg    *conf.AyatoConfig // planned to reduce dependency in the future
	s      service.Servicer
	allow  *auth.AllowlistRepo // nil when auth is not wired (tests)
	signer *auth.Signer        // nil when auth is not wired (tests)
}

func New(service service.Servicer, cfg *conf.AyatoConfig) *Handler {
	return &Handler{
		s:   service,
		cfg: cfg,
	}
}

// WithAuth attaches the admin allowlist and the stateless signer. The allowlist
// is the only server-side auth state; the signer mints/verifies sessions, CLI
// tokens, one-time codes, and OAuth state. Set during server startup; tests
// construct handlers without it.
func (h *Handler) WithAuth(allow *auth.AllowlistRepo, signer *auth.Signer) *Handler {
	h.allow = allow
	h.signer = signer
	return h
}
