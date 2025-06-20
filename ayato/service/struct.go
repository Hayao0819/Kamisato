package service

import (
	"github.com/Hayao0819/Kamisato/ayato/repository"
	"github.com/Hayao0819/Kamisato/internal/conf"
)

type Service struct {
	r   *repository.Repository
	cfg *conf.AyatoConfig
}

func New(repo *repository.Repository, config *conf.AyatoConfig) Service {
	return Service{
		r:   repo,
		cfg: config,
	}
}
