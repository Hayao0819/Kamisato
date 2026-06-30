package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/Hayao0819/Kamisato/internal/utils"
	"github.com/Hayao0819/Kamisato/miko/domain"
	"github.com/Hayao0819/Kamisato/pkg/pacman/builder"
)

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
			s.update(job.ID, func(j *domain.BuildJob) {
				j.Logs = job.Log.String()
				j.Log = nil
			})
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
	// Client sign mode retains the artifacts for download; every other path
	// cleans the dir up when process exits.
	keepArtifacts := err == nil && res != nil && job.Request.SignMode == domain.SignClient
	if outDir != "" && !keepArtifacts {
		defer func() { _ = os.RemoveAll(outDir) }()
	}

	// On terminal state, close the log buffer so SSE readers finish.
	defer func() {
		if job.Log != nil {
			job.Log.Close()
			s.update(job.ID, func(j *domain.BuildJob) {
				j.Logs = job.Log.String()
				j.Log = nil
			})
		}
	}()

	if err == nil && res != nil && job.Request.SignMode != domain.SignClient {
		if uerr := s.signAndUpload(jobCtx, job.Repo, res.Packages); uerr != nil {
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
			if keepArtifacts {
				j.ArtifactDir = outDir
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

	s.sweepArtifacts(artifactRetention)
}

// artifactRetention bounds how long a client-signed job's artifacts are kept for
// download; they are swept opportunistically as later jobs finish.
const artifactRetention = time.Hour

// sweepInterval is how often the worker sweeps expired artifacts independent of
// job completion, so an idle worker still releases disk within the retention
// window instead of waiting for the next job.
const sweepInterval = artifactRetention / 2

// sweepArtifacts removes retained artifact dirs of client jobs that ended longer
// than ttl ago.
func (s *Service) sweepArtifacts(ttl time.Duration) {
	now := time.Now()
	s.mu.Lock()
	var dirs []string
	for _, j := range s.store {
		if j.ArtifactDir != "" && j.EndedAt != nil && now.Sub(*j.EndedAt) > ttl {
			dirs = append(dirs, j.ArtifactDir)
			j.ArtifactDir = ""
		}
	}
	s.mu.Unlock()
	for _, d := range dirs {
		_ = os.RemoveAll(d)
	}
}

// sweepLoop sweeps expired artifact dirs on a timer until ctx is cancelled, so
// an idle worker reclaims disk without waiting for a later job to finish.
func (s *Service) sweepLoop(ctx context.Context, interval, ttl time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.sweepArtifacts(ttl)
		}
	}
}

// ArtifactDir returns the retained artifact directory for a client-signed job.
func (s *Service) ArtifactDir(id string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	job, ok := s.store[id]
	if !ok {
		return "", utils.NewErrf("job not found: %s", id)
	}
	if job.ArtifactDir == "" {
		return "", utils.NewErrf("job %s has no downloadable artifacts", id)
	}
	return job.ArtifactDir, nil
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

func isCancelled(ctx context.Context, err error) bool {
	return errors.Is(err, context.Canceled) || ctx.Err() == context.Canceled
}
