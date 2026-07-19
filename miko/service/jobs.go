package service

import (
	"context"
	"fmt"
	"log/slog"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/Hayao0819/Kamisato/internal/errors"
	"github.com/Hayao0819/Kamisato/miko/domain"
	"github.com/Hayao0819/Kamisato/pkg/pacman/builder"
	"github.com/Hayao0819/Kamisato/pkg/pacman/reponame"
)

// allowedArches are the architectures a build may target. Arch flows into shell
// and command construction in the backends, so reject anything else up front.
var allowedArches = map[string]bool{
	"x86_64": true, "aarch64": true, "armv7h": true,
	"i486": true, "i686": true, "pentium4": true,
}

// validateInstallPkgs rejects InstallPkgs outside the staging dir. Each entry is
// bind-mounted into the untrusted build, so an unchecked host path lets a caller
// exfiltrate arbitrary files (signing keys under data_dir, /etc, ssh keys).
func (s *Service) validateInstallPkgs(pkgs []string) error {
	if len(pkgs) == 0 {
		return nil
	}
	if s.cfg.DataDir == "" {
		return fmt.Errorf("%w: install_pkgs not accepted without a staging dir", ErrInvalidRequest)
	}
	base := filepath.Join(s.cfg.DataDir, "staging")
	for _, p := range pkgs {
		abs, err := filepath.Abs(p)
		if err != nil {
			return fmt.Errorf("%w: invalid install_pkgs path %q", ErrInvalidRequest, p)
		}
		rel, err := filepath.Rel(base, abs)
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return fmt.Errorf("%w: install_pkgs path escapes staging dir: %q", ErrInvalidRequest, p)
		}
	}
	return nil
}

func (s *Service) Submit(req *domain.BuildRequest) (string, error) {
	return s.submitWithReason(req, domain.ReasonManual)
}

// submitWithReason validates req and enqueues it tagged with reason. Submit and
// the automated triggers (version-update monitor, soname rebuild chain) share
// this path so every job runs the same validation.
func (s *Service) submitWithReason(req *domain.BuildRequest, reason domain.BuildReason) (string, error) {
	if req == nil {
		return "", fmt.Errorf("%w: request is nil", ErrInvalidRequest)
	}
	if !allowedArches[req.Arch] {
		return "", fmt.Errorf("%w: unsupported arch %q", ErrInvalidRequest, req.Arch)
	}
	if req.Repo != "" {
		if err := reponame.Validate(req.Repo); err != nil {
			return "", fmt.Errorf("%w: invalid repo name %q: %v", ErrInvalidRequest, req.Repo, err)
		}
	}
	if req.Microarch != "" {
		// Feature levels are an x86-64 concept, and an unknown tier must fail loudly
		// rather than silently building at the baseline.
		if req.Arch != "x86_64" {
			return "", fmt.Errorf("%w: microarch %q requires arch x86_64", ErrInvalidRequest, req.Microarch)
		}
		if !builder.ValidMicroarch(req.Microarch) {
			return "", fmt.Errorf("%w: unknown microarch tier %q", ErrInvalidRequest, req.Microarch)
		}
	}
	if err := s.validateInstallPkgs(req.InstallPkgs); err != nil {
		return "", err
	}

	job := &domain.BuildJob{
		ID:        newJobID(),
		Repo:      req.Repo,
		Arch:      req.Arch,
		Status:    domain.JobStatusQueued,
		Reason:    reason,
		Owner:     req.Requester,
		Request:   req,
		CreatedAt: time.Now(),
	}
	s.newLogBuffer(job.ID)

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
		// process() never runs for a rejected job, so finalize the log buffer here
		// or SSE readers of this job would block forever.
		s.finalizeLog(job.ID)
		s.setStatus(job.ID, domain.JobStatusFailed, err)
		return "", errors.WrapErr(err, "failed to enqueue build job")
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
		return nil, errors.NewErrf("job not found: %s", id)
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
		return errors.NewErrf("job not found: %s", id)
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
	var wg sync.WaitGroup
	start := func(run func()) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			run()
		}()
	}

	// Run owns every long-lived goroutine associated with the service. This gives
	// the composition root one completion signal to await during shutdown.
	start(func() { s.sweepLoop(ctx, sweepInterval, artifactRetention) })
	if interval := s.nvcheckInterval(); interval > 0 {
		start(func() { s.nvcheckLoop(ctx, interval) })
	}

	workers := max(s.cfg.Concurrency, 1)
	slog.Info("Build service started", "executor", s.cfg.Executor, "workers", workers)
	for range workers {
		start(func() { s.runWorker(ctx) })
	}
	wg.Wait()
	slog.Info("Build service stopped")
}

func (s *Service) runWorker(ctx context.Context) {
	for {
		job, ok := s.queue.pop(ctx)
		if !ok {
			return
		}
		s.process(ctx, job)
	}
}
