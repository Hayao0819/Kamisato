package handler

import (
	"github.com/Hayao0819/Kamisato/ayato/repository"
	"github.com/Hayao0819/Kamisato/conf"
)

// 最終的にServiceのみの依存にする
type Handler struct {
	cfg *conf.AyatoConfig // 間違った依存なのでいつか消す
	db  repository.Repository
}

func NewHandler(cfg *conf.AyatoConfig, db repository.Repository) *Handler {
	return &Handler{
		cfg: cfg,
		db:  db,
	}
}
