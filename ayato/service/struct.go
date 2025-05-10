package service

import (
	"github.com/Hayao0819/Kamisato/ayato/repository"
)

type Service struct {
	repo repository.Repository
}

func NewService(repo repository.Repository) Service {
	return Service{
		repo: repo,
	}
}
