package handler

import "github.com/Hayao0819/Kamisato/conf"

type Handler struct{
	cfg *conf.AyatoConfig
}

func NewHandler(cfg *conf.AyatoConfig) *Handler {
	return &Handler{
		cfg: cfg,
	}
}

