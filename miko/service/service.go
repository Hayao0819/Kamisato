package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"sort"
	"sync"
	"time"

	"github.com/Hayao0819/Kamisato/internal/conf"
	"github.com/Hayao0819/Kamisato/internal/utils"
	"github.com/Hayao0819/Kamisato/miko/domain"
	"github.com/Hayao0819/Kamisato/miko/joblog"
	"github.com/Hayao0819/Kamisato/pkg/pacman/builder"
)

// Sentinel errors let the handler map Submit failures to HTTP status codes.
var (
	// ErrInvalidRequest is a client error (bad arch, nil request) -> 400.
	ErrInvalidRequest = errors.New("invalid build request")
	// ErrQueueFull means the worker is saturated -> 503.
	ErrQueueFull = errors.New("build queue is full")
)

// IService is the interface exposed by the miko build service.
//
//go:generate mockgen -source=service.go -destination=../test/mocks/service.go -package=mocks
type IService interface {
	// Submit enqueues a build request and returns the assigned job ID.
	Submit(req *domain.BuildRequest) (string, error)
	// Status returns the current state of a job.
	Status(id string) (*domain.BuildJob, error)
	// List returns all jobs, newest first.
	List() []*domain.BuildJob
	// Run is the worker loop. It blocks until ctx is cancelled.
	Run(ctx context.Context)
}

// Service holds the build configuration, the job queue and an in-memory job
// store backed by optional on-disk persistence.
type Service struct {
	cfg     *conf.MikoConfig
	queue   *queue
	persist *jobPersist

	mu    sync.Mutex
	store map[string]*domain.BuildJob
}

func New(cfg *conf.MikoConfig) IService {
	s := &Service{
		cfg:   cfg,
		queue: newQueue(),
		store: make(map[string]*domain.BuildJob),
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

// allowedArches are the architectures a build may target. Arch flows into shell
// and command construction in the backends, so reject anything else up front.
var allowedArches = map[string]bool{"x86_64": true, "aarch64": true, "armv7h": true}

func (s *Service) Submit(req *domain.BuildRequest) (string, error) {
	if req == nil {
		return "", fmt.Errorf("%w: request is nil", ErrInvalidRequest)
	}
	if !allowedArches[req.Arch] {
		return "", fmt.Errorf("%w: unsupported arch %q", ErrInvalidRequest, req.Arch)
	}

	job := &domain.BuildJob{
		ID:        newJobID(),
		Repo:      req.Repo,
		Arch:      req.Arch,
		Status:    domain.JobStatusQueued,
		Request:   req,
		CreatedAt: time.Now(),
		Log:       joblog.New(),
	}

	s.mu.Lock()
	s.store[job.ID] = job
	evicted := s.evictLocked()
	snap := *job
	s.mu.Unlock()
	s.persistSave(&snap)
	for _, id := range evicted {
		s.persistRemove(id)
	}

	if err := s.queue.push(job); err != nil {
		// process() never runs for a rejected job, so close the log buffer here
		// or SSE readers of this job would block forever.
		job.Log.Close()
		s.setStatus(job.ID, domain.JobStatusFailed, err)
		return "", utils.WrapErr(err, "failed to enqueue build job")
	}

	slog.Info("Build job submitted", "id", job.ID, "repo", job.Repo, "arch", job.Arch)
	return job.ID, nil
}

// Status returns a copy of the requested job.
func (s *Service) Status(id string) (*domain.BuildJob, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	job, ok := s.store[id]
	if !ok {
		return nil, utils.NewErrf("job not found: %s", id)
	}
	clone := *job
	return &clone, nil
}

// List returns clones of every job, sorted by CreatedAt descending.
func (s *Service) List() []*domain.BuildJob {
	s.mu.Lock()
	defer s.mu.Unlock()

	jobs := make([]*domain.BuildJob, 0, len(s.store))
	for _, job := range s.store {
		clone := *job
		jobs = append(jobs, &clone)
	}
	sort.Slice(jobs, func(i, j int) bool {
		return jobs[i].CreatedAt.After(jobs[j].CreatedAt)
	})
	return jobs
}

func (s *Service) Run(ctx context.Context) {
	slog.Info("Build worker started", "executor", s.cfg.Executor)
	for {
		job, ok := s.queue.pop(ctx)
		if !ok {
			slog.Info("Build worker stopping")
			return
		}
		s.process(ctx, job)
	}
}

func (s *Service) process(ctx context.Context, job *domain.BuildJob) {
	now := time.Now()
	s.update(job.ID, func(j *domain.BuildJob) {
		j.Status = domain.JobStatusRunning
		j.StartedAt = &now
	})
	slog.Info("Build job running", "id", job.ID)

	res, outDir, err := s.runBuild(ctx, job)
	// Keep the artifact directory until signing and upload, then clean it up
	// all at once when process exits.
	if outDir != "" {
		defer func() { _ = os.RemoveAll(outDir) }()
	}

	// When a job reaches a terminal state, always close the log buffer to let
	// SSE readers finish. Also write the final log out to Logs.
	defer func() {
		if job.Log != nil {
			job.Log.Close()
			s.update(job.ID, func(j *domain.BuildJob) { j.Logs = job.Log.String() })
		}
	}()

	if err == nil && res != nil {
		if uerr := signAndUpload(s.cfg, job.Repo, job.Request.GPGKey, res.Packages); uerr != nil {
			err = utils.WrapErr(uerr, "sign and upload failed")
		}
	}

	end := time.Now()
	s.update(job.ID, func(j *domain.BuildJob) {
		j.EndedAt = &end
		if err != nil {
			j.Status = domain.JobStatusFailed
			j.Err = err.Error()
			return
		}
		j.Status = domain.JobStatusSuccess
		if res != nil {
			j.Packages = res.Packages
		}
	})

	if err != nil {
		slog.Error("Build job failed", "id", job.ID, "error", err)
		return
	}
	slog.Info("Build job succeeded", "id", job.ID, "packages", len(res.Packages))
}

// runBuild prepares a build Spec, invokes the backend and captures logs.
// On success it returns the output directory holding the built packages; the
// caller owns its cleanup (after signing/uploading).
func (s *Service) runBuild(ctx context.Context, job *domain.BuildJob) (*builder.Result, string, error) {
	req := job.Request

	// Disposable source directory (discarded after the build).
	srcDir, err := os.MkdirTemp("", "miko-src-*")
	if err != nil {
		return nil, "", utils.WrapErr(err, "failed to create source dir")
	}
	defer func() { _ = os.RemoveAll(srcDir) }()

	if err := materialize(req, srcDir); err != nil {
		return nil, "", utils.WrapErr(err, "failed to materialize source")
	}

	// The artifact directory must live beyond runBuild (until signing and
	// upload), so do not remove it here. Clean it up only on failure.
	outDir, err := os.MkdirTemp("", "miko-out-*")
	if err != nil {
		return nil, "", utils.WrapErr(err, "failed to create output dir")
	}

	timeout := time.Duration(s.cfg.Build.Timeout) * time.Minute
	backend, err := builder.New(builder.Kind(s.cfg.Executor), builder.Options{
		Image:      s.cfg.Build.Image,
		Timeout:    timeout,
		DockerHost: s.cfg.DockerHost,
	})
	if err != nil {
		_ = os.RemoveAll(outDir)
		return nil, "", utils.WrapErr(err, "failed to create build backend")
	}

	spec := builder.Spec{
		SrcDir:      srcDir,
		OutDir:      outDir,
		Arch:        req.Arch,
		ArchBuild:   s.archBuildFor(req.Arch),
		InstallPkgs: req.InstallPkgs,
		LogWriter:   job.Log,
	}

	res, err := backend.Build(ctx, spec)
	if err != nil {
		_ = os.RemoveAll(outDir)
		return nil, "", utils.WrapErr(err, "build failed")
	}
	return res, outDir, nil
}

// archBuildFor maps a CARCH to the devtools wrapper used by the chroot backend,
// using the configured ArchBuildTemplate (default "extra-%s-build").
func (s *Service) archBuildFor(arch string) string {
	if arch == "" {
		return ""
	}
	tmpl := s.cfg.ArchBuildTemplate
	if tmpl == "" {
		tmpl = "extra-%s-build"
	}
	return fmt.Sprintf(tmpl, arch)
}

func (s *Service) setStatus(id string, status domain.JobStatus, err error) {
	s.update(id, func(j *domain.BuildJob) {
		j.Status = status
		if err != nil {
			j.Err = err.Error()
		}
	})
}

func newJobID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		// Fall back to a time-based ID; collisions are improbable.
		return hex.EncodeToString([]byte(time.Now().Format("20060102150405.000000000")))
	}
	return hex.EncodeToString(b[:])
}

func (s *Service) update(id string, fn func(*domain.BuildJob)) {
	s.mu.Lock()
	job, ok := s.store[id]
	if ok {
		fn(job)
	}
	var snap *domain.BuildJob
	if ok {
		c := *job
		snap = &c
	}
	s.mu.Unlock()
	if snap != nil {
		s.persistSave(snap)
	}
}

// maxStoredJobs caps the in-memory job store. The store is not persisted, so
// this only bounds memory; older terminal jobs are dropped first.
const maxStoredJobs = 500

// evictLocked drops the oldest terminal (success/failed) jobs until the store
// is within maxStoredJobs, returning the evicted IDs so the caller can remove
// them from disk. Queued/running jobs are never evicted. Callers must hold s.mu.
func (s *Service) evictLocked() []string {
	if len(s.store) <= maxStoredJobs {
		return nil
	}
	terminal := make([]*domain.BuildJob, 0, len(s.store))
	for _, j := range s.store {
		if j.Status == domain.JobStatusSuccess || j.Status == domain.JobStatusFailed {
			terminal = append(terminal, j)
		}
	}
	sort.Slice(terminal, func(i, j int) bool {
		return terminal[i].CreatedAt.Before(terminal[j].CreatedAt)
	})
	var evicted []string
	for _, j := range terminal {
		if len(s.store) <= maxStoredJobs {
			break
		}
		delete(s.store, j.ID)
		evicted = append(evicted, j.ID)
	}
	return evicted
}
