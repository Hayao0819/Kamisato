package service

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/Hayao0819/Kamisato/internal/conf"
	"github.com/Hayao0819/Kamisato/miko/domain"
)

// Sentinel errors let the handler map Submit failures to HTTP status codes.
var (
	// ErrInvalidRequest is a client error (bad arch, nil request) -> 400.
	ErrInvalidRequest = errors.New("invalid build request")
	// ErrQueueFull means the worker is saturated -> 503.
	ErrQueueFull = errors.New("build queue is full")
	// ErrJobNotCancelable means the job is already terminal -> 409.
	ErrJobNotCancelable = errors.New("job is already terminal")
)

// Servicer is the interface exposed by the miko build service.
//
//go:generate mockgen -source=service.go -destination=../test/mocks/service.go -package=mocks
type Servicer interface {
	// Submit enqueues a build request and returns the assigned job ID.
	Submit(req *domain.BuildRequest) (string, error)
	Status(id string) (*domain.BuildJob, error)
	// List returns all jobs, newest first.
	List() []*domain.BuildJob
	Cancel(id string) error
	Stats() domain.BuildStats
	// Run is the worker loop. It blocks until ctx is cancelled.
	Run(ctx context.Context)
}

// Service holds the build configuration, the job queue and an in-memory job
// store backed by optional on-disk persistence.
type Service struct {
	cfg     *conf.MikoConfig
	queue   *queue
	persist *jobPersist

	startedAt time.Time

	mu    sync.Mutex
	store map[string]*domain.BuildJob
	// running maps in-flight job IDs to their cancel func (guarded by mu).
	running map[string]context.CancelFunc
}

func New(cfg *conf.MikoConfig) Servicer {
	s := &Service{
		cfg:       cfg,
		queue:     newQueue(),
		store:     make(map[string]*domain.BuildJob),
		running:   make(map[string]context.CancelFunc),
		startedAt: time.Now(),
	}
	if cfg.DataDir != "" {
		p, err := newJobPersist(cfg.DataDir)
		if err != nil {
			slog.Error("job persistence disabled", "error", err)
		} else {
			s.persist = p
			s.restore()
		}
	}
	return s
}

// restore loads persisted jobs into the store. A job left queued/running by a
// crash is marked failed, since the in-memory queue does not survive a restart.
// Runs before the worker starts, so no locking is needed.
func (s *Service) restore() {
	jobs, err := s.persist.loadAll()
	if err != nil {
		slog.Error("failed to load persisted jobs", "error", err)
		return
	}
	for _, job := range jobs {
		if job.Status == domain.JobStatusQueued || job.Status == domain.JobStatusRunning {
			job.Status = domain.JobStatusFailed
			if job.Err == "" {
				job.Err = "interrupted by server restart"
			}
			_ = s.persist.save(job)
		}
		s.store[job.ID] = job
	}
	slog.Info("restored persisted jobs", "count", len(jobs))
}

func (s *Service) persistSave(job *domain.BuildJob) {
	if s.persist == nil {
		return
	}
	if err := s.persist.save(job); err != nil {
		slog.Warn("failed to persist job", "id", job.ID, "error", err)
	}
}

func (s *Service) persistRemove(id string) {
	if s.persist == nil {
		return
	}
	if err := s.persist.remove(id); err != nil {
		slog.Warn("failed to remove persisted job", "id", id, "error", err)
	}
}
