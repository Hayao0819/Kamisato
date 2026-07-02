package service

import (
	"context"
	"fmt"
	"log/slog"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/Hayao0819/Kamisato/internal/errwrap"
	"github.com/Hayao0819/Kamisato/miko/domain"
)

// allowedArches are the architectures a build may target. Arch flows into shell
// and command construction in the backends, so reject anything else up front.
var allowedArches = map[string]bool{"x86_64": true, "aarch64": true, "armv7h": true}

// repoNamePattern bounds the repo name to a pacman-safe charset. Repo flows into
// the upload target and, with resolve_aur_deps, into the generated build script's
// pacman.conf, so a newline or shell metacharacter must be rejected up front.
var repoNamePattern = regexp.MustCompile(`^[a-zA-Z0-9._-]+$`)

// validateInstallPkgs rejects InstallPkgs that point outside the staging dir.
// Each entry is bind-mounted into the untrusted build, so an unchecked host
// path lets a caller exfiltrate arbitrary files (signing keys under data_dir,
// /etc, ssh keys). miko has no upload endpoint for deps: operators stage them
// under <data_dir>/staging, and remote callers upload deps as source files.
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
	if req == nil {
		return "", fmt.Errorf("%w: request is nil", ErrInvalidRequest)
	}
	if !allowedArches[req.Arch] {
		return "", fmt.Errorf("%w: unsupported arch %q", ErrInvalidRequest, req.Arch)
	}
	if req.Repo != "" && !repoNamePattern.MatchString(req.Repo) {
		return "", fmt.Errorf("%w: invalid repo name %q", ErrInvalidRequest, req.Repo)
	}
	if err := s.validateInstallPkgs(req.InstallPkgs); err != nil {
		return "", err
	}

	job := &domain.BuildJob{
		ID:        newJobID(),
		Repo:      req.Repo,
		Arch:      req.Arch,
		Status:    domain.JobStatusQueued,
		Reason:    domain.ReasonManual,
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
		return "", errwrap.WrapErr(err, "failed to enqueue build job")
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
		return nil, errwrap.NewErrf("job not found: %s", id)
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
		return errwrap.NewErrf("job not found: %s", id)
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
	slog.Info("Build worker started", "executor", s.cfg.Executor)

	// Only the first worker starts the sweep loop; the rest would be redundant
	// tickers over the same shared store.
	var wg sync.WaitGroup
	s.sweepOnce.Do(func() {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s.sweepLoop(ctx, sweepInterval, artifactRetention)
		}()
	})
	defer wg.Wait()

	for {
		job, ok := s.queue.pop(ctx)
		if !ok {
			slog.Info("Build worker stopping")
			return
		}
		s.process(ctx, job)
	}
}
