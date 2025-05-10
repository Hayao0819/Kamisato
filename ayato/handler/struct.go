package handler

import (
	"github.com/Hayao0819/Kamisato/ayato/service"
	"github.com/Hayao0819/Kamisato/conf"
)

// 最終的にServiceのみの依存にする
type Handler struct {
	// cfg *conf.AyatoConfig // 間違った依存なのでいつか消す
	s service.Service
}

func NewHandler(cfg *conf.AyatoConfig, service service.Service) *Handler {
	return &Handler{
		// cfg: cfg,
		s: service,
	}
}
