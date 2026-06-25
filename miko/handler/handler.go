package handler

import (
	"sync"

	"github.com/Hayao0819/Kamisato/internal/conf"
	"github.com/Hayao0819/Kamisato/miko/service"
)

// Handler serves the miko build API.
type Handler struct {
	cfg *conf.MikoConfig
	s   service.Servicer

	// logReadersMu guards logReaders, the per-job count of in-flight SSE log
	// readers used to cap concurrent streams (see JobLogsHandler).
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
