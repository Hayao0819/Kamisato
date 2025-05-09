package handler

import (
	"github.com/Hayao0819/Kamisato/ayato/repository"
	"github.com/Hayao0819/Kamisato/conf"
)

type Handler struct {
	cfg *conf.AyatoConfig
	db  repository.PkgNameStoreProvider
}

func NewHandler(cfg *conf.AyatoConfig, db repository.PkgNameStoreProvider) *Handler {
	return &Handler{
		cfg: cfg,
	}
}
