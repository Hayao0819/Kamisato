package handler

import (
	"sync"

	"github.com/Hayao0819/Kamisato/internal/conf"
	"github.com/Hayao0819/Kamisato/miko/service"
)

type Handler struct {
	cfg *conf.MikoConfig
	s   service.Servicer

	// logReadersMu guards logReaders, the per-job in-flight SSE reader count used to cap concurrent streams.
	logReadersMu sync.Mutex
	logReaders   map[string]int
}

func New(s service.Servicer, cfg *conf.MikoConfig) *Handler {
	return &Handler{
		s:          s,
		cfg:        cfg,
		logReaders: make(map[string]int),
	}
}
