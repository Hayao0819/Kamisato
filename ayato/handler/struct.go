package handler

import (
	"github.com/Hayao0819/Kamisato/ayato/service"
	"github.com/Hayao0819/Kamisato/internal/conf"
)

// 最終的にServiceのみの依存にする
type Handler struct {
	cfg *conf.AyatoConfig // 間違った依存なので/いつか消す -> 別に良いらしい？
	s   service.Service
}

func New(service service.Service, cfg *conf.AyatoConfig) *Handler {
	return &Handler{
		s:   service,
		cfg: cfg,
	}
}
