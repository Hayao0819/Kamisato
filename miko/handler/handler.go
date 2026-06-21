package handler

import (
	"github.com/Hayao0819/Kamisato/internal/conf"
	"github.com/Hayao0819/Kamisato/miko/service"
)

// Handler serves the miko build API.
type Handler struct {
	cfg *conf.MikoConfig
	s   service.Servicer
}

func New(s service.Servicer, cfg *conf.MikoConfig) *Handler {
	return &Handler{
		s:   s,
		cfg: cfg,
	}
}
