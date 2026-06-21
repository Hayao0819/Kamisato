package handler

import (
	"github.com/Hayao0819/Kamisato/internal/conf"
	"github.com/Hayao0819/Kamisato/miko/service"
)

// Handler serves the miko build API.
type Handler struct {
	cfg *conf.MikoConfig
	s   service.IService
}

// New is the constructor for Handler.
func New(s service.IService, cfg *conf.MikoConfig) *Handler {
	return &Handler{
		s:   s,
		cfg: cfg,
	}
}
