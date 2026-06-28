package handler

import (
	"github.com/Hayao0819/Kamisato/ayato/auth"
	"github.com/Hayao0819/Kamisato/ayato/service"
	"github.com/Hayao0819/Kamisato/internal/conf"
)

type Handler struct {
	cfg    *conf.AyatoConfig
	s      service.Servicer
	signer *auth.Signer // nil when auth is not wired (tests)
}

func New(service service.Servicer, cfg *conf.AyatoConfig) *Handler {
	return &Handler{
		s:   service,
		cfg: cfg,
	}
}

// WithAuth attaches the stateless signer; set at startup, tests omit it (signer stays nil).
func (h *Handler) WithAuth(signer *auth.Signer) *Handler {
	h.signer = signer
	return h
}
