package handler

import (
	"github.com/Hayao0819/Kamisato/ayato/service"
)

// 最終的にServiceのみの依存にする
type Handler struct {
	// cfg *conf.AyatoConfig // 間違った依存なのでいつか消す
	s service.Service
}

func New(service service.Service) *Handler {
	return &Handler{
		s: service,
	}
}
