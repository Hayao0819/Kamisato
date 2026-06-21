package handler

import (
	"github.com/Hayao0819/Kamisato/ayato/service"
	"github.com/Hayao0819/Kamisato/internal/conf"
)

// Handler is a struct that handles API requests.
// Eventually planned to depend only on Service.
type Handler struct {
	cfg *conf.AyatoConfig // planned to reduce dependency in the future
	s   service.Servicer
}

func New(service service.Servicer, cfg *conf.AyatoConfig) *Handler {
	return &Handler{
		s:   service,
		cfg: cfg,
	}
}
