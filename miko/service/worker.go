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
