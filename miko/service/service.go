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
	// ErrJobNotCancelable means the job is already terminal -> 409.
	ErrJobNotCancelable = errors.New("job is already terminal")
)

// Servicer is the interface exposed by the miko build service.
//
//go:generate mockgen -source=service.go -destination=../test/mocks/service.go -package=mocks
type Servicer interface {
	// Submit enqueues a build request and returns the assigned job ID.
	Submit(req *domain.BuildRequest) (string, error)
	// Status returns the current state of a job.
	Status(id string) (*domain.BuildJob, error)
	// List returns all jobs, newest first.
	List() []*domain.BuildJob
	// Cancel cancels a queued or running job.
	Cancel(id string) error
	// Stats returns a snapshot of the build service.
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

// Cancel marks a queued job cancelled (the worker skips it when popped) or
// cancels a running job's build context. Terminal jobs return ErrJobNotCancelable.
func (s *Service) Cancel(id string) error {
	s.mu.Lock()
	job, ok := s.store[id]
	if !ok {
		s.mu.Unlock()
		return utils.NewErrf("job not found: %s", id)
	}
	switch job.Status {
	case domain.JobStatusSuccess, domain.JobStatusFailed, domain.JobStatusCancelled:
		s.mu.Unlock()
		return ErrJobNotCancelable
	case domain.JobStatusQueued:
		job.Status = domain.JobStatusCancelled
		end := time.Now()
		job.EndedAt = &end
		snap := *job
		s.mu.Unlock()
		s.persistSave(&snap)
		slog.Info("Build job cancelled while queued", "id", id)
		return nil
	default:
		// Running: the worker finalizes the cancelled status once the build stops.
		cancel := s.running[id]
		s.mu.Unlock()
		if cancel != nil {
			cancel()
		}
		slog.Info("Build job cancellation requested", "id", id)
		return nil
	}
}

func (s *Service) Stats() domain.BuildStats {
	s.mu.Lock()
	counts := make(map[domain.JobStatus]int)
	for _, j := range s.store {
		counts[j.Status]++
	}
	total := len(s.store)
	s.mu.Unlock()

	workers := s.cfg.Concurrency
	if workers < 1 {
		workers = 1
	}
	success := counts[domain.JobStatusSuccess]
	failed := counts[domain.JobStatusFailed]
	var rate float64
	if finished := success + failed; finished > 0 {
		rate = float64(success) / float64(finished)
	}
	return domain.BuildStats{
		Workers:     workers,
		QueueLength: s.queue.len(),
		Running:     counts[domain.JobStatusRunning],
		Counts:      counts,
		Total:       total,
		SuccessRate: rate,
		UptimeSec:   int(time.Since(s.startedAt).Seconds()),
	}
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
	// Per-job context so Cancel can stop this build; registered under mu, cleared on return.
	jobCtx, cancel := context.WithCancel(ctx)
	s.mu.Lock()
	// Skip a job cancelled while still queued.
	if cur, ok := s.store[job.ID]; ok && cur.Status == domain.JobStatusCancelled {
		s.mu.Unlock()
		cancel()
		if job.Log != nil {
			job.Log.Close()
			s.update(job.ID, func(j *domain.BuildJob) { j.Logs = job.Log.String() })
		}
		slog.Info("Skipping cancelled job", "id", job.ID)
		return
	}
	s.running[job.ID] = cancel
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		delete(s.running, job.ID)
		s.mu.Unlock()
		cancel()
	}()

	now := time.Now()
	s.update(job.ID, func(j *domain.BuildJob) {
		j.Status = domain.JobStatusRunning
		j.StartedAt = &now
	})
	slog.Info("Build job running", "id", job.ID)

	res, outDir, err := s.buildWithRetry(jobCtx, job)
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

	cancelled := isCancelled(jobCtx, err)
	end := time.Now()
	s.update(job.ID, func(j *domain.BuildJob) {
		j.EndedAt = &end
		switch {
		case cancelled:
			j.Status = domain.JobStatusCancelled
			if err != nil {
				j.Err = err.Error()
			}
		case err != nil:
			j.Status = domain.JobStatusFailed
			j.Err = err.Error()
		default:
			j.Status = domain.JobStatusSuccess
			if res != nil {
				j.Packages = res.Packages
			}
		}
	})

	switch {
	case cancelled:
		slog.Info("Build job cancelled", "id", job.ID)
	case err != nil:
		slog.Error("Build job failed", "id", job.ID, "error", err)
	default:
		slog.Info("Build job succeeded", "id", job.ID, "packages", len(res.Packages))
	}
}

// buildWithRetry runs the build with retry/backoff, retrying only on a genuine
// failure while attempts remain and the context is live.
func (s *Service) buildWithRetry(ctx context.Context, job *domain.BuildJob) (*builder.Result, string, error) {
	maxRetries := s.cfg.MaxRetries
	if maxRetries < 0 {
		maxRetries = 0
	}
	backoff := time.Duration(s.cfg.RetryBackoff) * time.Second

	var (
		res    *builder.Result
		outDir string
		err    error
	)
	for attempt := 0; ; attempt++ {
		res, outDir, err = s.runBuild(ctx, job)
		if err == nil {
			return res, outDir, nil
		}
		// Do not retry on cancellation or when the loop is out of attempts.
		if isCancelled(ctx, err) || attempt >= maxRetries {
			return res, outDir, err
		}

		line := fmt.Sprintf("retry %d/%d after error: %v\n", attempt+1, maxRetries, err)
		if job.Log != nil {
			_, _ = job.Log.Write([]byte(line))
		}
		s.update(job.ID, func(j *domain.BuildJob) { j.Retries = attempt + 1 })

		if backoff > 0 {
			t := time.NewTimer(backoff)
			select {
			case <-ctx.Done():
				t.Stop()
				return res, outDir, err
			case <-t.C:
			}
		}
	}
}

// isCancelled distinguishes a context cancellation from a genuine build failure.
func isCancelled(ctx context.Context, err error) bool {
	return errors.Is(err, context.Canceled) || ctx.Err() == context.Canceled
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

	// Per-request timeout (minutes) overrides the server default.
	timeoutMin := s.cfg.Build.Timeout
	if req.Timeout > 0 {
		timeoutMin = req.Timeout
	}
	timeout := time.Duration(timeoutMin) * time.Minute

	opts := builder.Options{
		Image:      s.cfg.Build.Image,
		Timeout:    timeout,
		DockerHost: s.cfg.DockerHost,
	}
	if s.cfg.Cache.Enabled {
		opts.PacmanCacheDir = s.cfg.Cache.PacmanCacheDir
		opts.CcacheDir = s.cfg.Cache.CcacheDir
	}
	backend, err := builder.New(builder.Kind(s.cfg.Executor), opts)
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
