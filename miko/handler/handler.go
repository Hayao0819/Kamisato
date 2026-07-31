package handler

import (
	"sync"

	"github.com/Hayao0819/Kamisato/miko/service"
)

type Settings struct {
	MaxLogReaders int
}

type Handler struct {
	settings Settings
	s        service.Servicer

	// logReadersMu guards logReaders, the per-job in-flight SSE reader count used to cap concurrent streams.
	logReadersMu sync.Mutex
	logReaders   map[string]int
}

func New(s service.Servicer, settings Settings) *Handler {
	if settings.MaxLogReaders <= 0 {
		settings.MaxLogReaders = 8
	}
	return &Handler{
		s:          s,
		settings:   settings,
		logReaders: make(map[string]int),
	}
}
