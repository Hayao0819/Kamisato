package service

import (
	"github.com/Hayao0819/Kamisato/ayato/repository"
)

type Service struct {
	r *repository.Repository
}

func New(repo *repository.Repository) Service {
	return Service{
		r: repo,
	}
}
