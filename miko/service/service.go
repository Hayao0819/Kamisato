package service

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/Hayao0819/Kamisato/internal/errors"

	"github.com/Hayao0819/Kamisato/internal/conf"
	"github.com/Hayao0819/Kamisato/miko/domain"
	"github.com/Hayao0819/Kamisato/miko/joblog"
	"github.com/Hayao0819/Kamisato/pkg/pacman/sign"
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

//go:generate mockgen -source=service.go -destination=../test/mocks/service.go -package=mocks
type Servicer interface {
	Submit(req *domain.BuildRequest) (string, error)
	Status(id string) (*domain.BuildJob, error)
	// List returns all jobs, newest first.
	List() []*domain.BuildJob
	Cancel(id string) error
	Stats() domain.BuildStats
	// ArtifactDir returns the retained artifact directory of a client-signed job.
	ArtifactDir(id string) (string, error)
	// LogBuffer returns the live log buffer of an in-flight job, or nil when the
	// job has no live buffer (finalized, restored from disk, or unknown).
	LogBuffer(id string) *joblog.Buffer
	// Run is the worker loop. It blocks until ctx is cancelled.
	Run(ctx context.Context)
}

type Service struct {
	cfg      *conf.MikoConfig
	signer   sign.Signer // host signing key; nil disables signing
	queue    *queue
	persist  Persister   // durable job store; nil disables persistence
	uploader Uploader    // publishes built packages to ayato
	sonames  sonameStore // per-pkgbase soname history; nil disables bump detection

	startedAt time.Time

	// sweepOnce ensures a single artifact-sweep loop runs even when several
	// worker goroutines share this Service (cfg.Concurrency > 1).
	sweepOnce sync.Once
	// nvcheckOnce ensures a single upstream-version monitor loop runs regardless
	// of the worker count.
	nvcheckOnce sync.Once

	mu    sync.Mutex
	store map[string]*domain.BuildJob
	// running maps in-flight job IDs to their cancel func (guarded by mu).
	running map[string]context.CancelFunc

	// logs holds each in-flight job's live log buffer, keyed by job ID and guarded
	// by logsMu. It is kept out of the domain job so status snapshots never share a
	// live buffer.
	logsMu sync.Mutex
	logs   map[string]*joblog.Buffer
}

// New builds the service with its collaborators injected: persister durably
// stores jobs (nil disables persistence) and uploader publishes built packages.
func New(cfg *conf.MikoConfig, signer sign.Signer, persister Persister, uploader Uploader) Servicer {
	s := &Service{
		cfg:       cfg,
		signer:    signer,
		queue:     newQueue(),
		persist:   persister,
		uploader:  uploader,
		store:     make(map[string]*domain.BuildJob),
		running:   make(map[string]context.CancelFunc),
		logs:      make(map[string]*joblog.Buffer),
		startedAt: time.Now(),
	}
	if persister != nil {
		s.restore()
	}
	// Soname history is small per-pkgbase state; persist it under the data dir so
	// a bump is detected across restarts. Disabled (nil) without a data dir.
	if cfg.DataDir != "" {
		if st, err := newFileSonameStore(cfg.DataDir); err != nil {
			slog.Warn("soname history disabled", "error", err)
		} else {
			s.sonames = st
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

func (s *Service) newLogBuffer(id string) *joblog.Buffer {
	b := joblog.New(s.cfg.MaxLogBytes)
	s.logsMu.Lock()
	s.logs[id] = b
	s.logsMu.Unlock()
	return b
}

// LogBuffer returns the live log buffer for id, or nil once the job is finalized.
func (s *Service) LogBuffer(id string) *joblog.Buffer {
	s.logsMu.Lock()
	defer s.logsMu.Unlock()
	return s.logs[id]
}

// finalizeLog closes a job's live buffer (letting SSE readers finish) and
// snapshots its text into the durable job record so later reads still see the
// logs. It is a no-op when there is no live buffer.
func (s *Service) finalizeLog(id string) {
	s.logsMu.Lock()
	b := s.logs[id]
	delete(s.logs, id)
	s.logsMu.Unlock()
	if b == nil {
		return
	}
	b.Close()
	text := b.String()
	s.update(id, func(j *domain.BuildJob) { j.Logs = text })
}
