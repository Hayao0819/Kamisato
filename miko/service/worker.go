package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/Hayao0819/Kamisato/internal/errwrap"
	"github.com/Hayao0819/Kamisato/miko/domain"
	"github.com/Hayao0819/Kamisato/pkg/pacman/builder"
	"github.com/cenkalti/backoff/v5"
)

func (s *Service) process(ctx context.Context, job *domain.BuildJob) {
	// Per-job context so Cancel can stop this build; registered under mu, cleared on return.
	jobCtx, cancel := context.WithCancel(ctx)
	s.mu.Lock()
	// Skip a job cancelled while still queued.
	if cur, ok := s.store[job.ID]; ok && cur.Status == domain.JobStatusCancelled {
		s.mu.Unlock()
		cancel()
		s.finalizeLog(job.ID)
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

	// On terminal state, finalize the log buffer so SSE readers finish.
	defer s.finalizeLog(job.ID)

	if err == nil && res != nil && job.Request.SignMode != domain.SignClient {
		if uerr := s.signAndUpload(jobCtx, job.Repo, res.Packages); uerr != nil {
			err = errwrap.WrapErr(uerr, "sign and upload failed")
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
// download; they are swept on a timer (sweepInterval) and as jobs finish.
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
	var swept []domain.BuildJob
	for _, j := range s.store {
		if j.ArtifactDir != "" && j.EndedAt != nil && now.Sub(*j.EndedAt) > ttl {
			dirs = append(dirs, j.ArtifactDir)
			j.ArtifactDir = ""
			swept = append(swept, *j)
		}
	}
	s.mu.Unlock()
	for _, d := range dirs {
		_ = os.RemoveAll(d)
	}
	// Persist the cleared dir so a restart does not restore a job pointing at a
	// directory that no longer exists.
	for i := range swept {
		s.persistSave(&swept[i])
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
		return "", errwrap.NewErrf("job not found: %s", id)
	}
	if job.ArtifactDir == "" {
		return "", errwrap.NewErrf("job %s has no downloadable artifacts", id)
	}
	return job.ArtifactDir, nil
}

// buildOutcome bundles runBuild's two success values so backoff.Retry can carry
// them through its single generic result type.
type buildOutcome struct {
	res    *builder.Result
	outDir string
}

// buildWithRetry runs the build, retrying a genuine failure up to MaxRetries
// times with exponential backoff and jitter. Cancellation is terminal and does
// not consume a retry.
func (s *Service) buildWithRetry(ctx context.Context, job *domain.BuildJob) (*builder.Result, string, error) {
	maxRetries := s.cfg.MaxRetries
	if maxRetries < 0 {
		maxRetries = 0
	}

	exp := backoff.NewExponentialBackOff()
	exp.InitialInterval = time.Duration(s.cfg.RetryBackoff) * time.Second

	attempt := 0
	// notify only fires before a genuine retry (a terminal failure returns a
	// Permanent error, which backoff does not notify on), so tagging the job here
	// records that its current attempt is a retry of a transient failure.
	notify := func(err error, _ time.Duration) {
		attempt++
		if buf := s.LogBuffer(job.ID); buf != nil {
			_, _ = buf.Write([]byte(fmt.Sprintf("retry %d/%d after retriable error: %v\n", attempt, maxRetries, err)))
		}
		s.update(job.ID, func(j *domain.BuildJob) {
			j.Retries = attempt
			j.Reason = domain.ReasonRetry
		})
	}

	op := func() (buildOutcome, error) {
		res, outDir, err := s.runBuild(ctx, job)
		// Cancellation and deterministic build failures (a compile/PKGBUILD error)
		// gain nothing from a retry, so stop immediately; only transient failures
		// (clone, image pull, network, a build timeout) fall through to backoff.
		if err != nil && (isCancelled(ctx, err) || !isRetriable(err)) {
			return buildOutcome{res, outDir}, backoff.Permanent(err)
		}
		return buildOutcome{res, outDir}, err
	}

	out, err := backoff.Retry(ctx, op,
		backoff.WithBackOff(exp),
		backoff.WithMaxTries(uint(maxRetries)+1), //nolint:gosec // maxRetries is clamped to >= 0 above
		backoff.WithMaxElapsedTime(0),            // no total-time cap: honor the configured retry count
		backoff.WithNotify(notify),
	)
	return out.res, out.outDir, err
}

func isCancelled(ctx context.Context, err error) bool {
	return errors.Is(err, context.Canceled) || ctx.Err() == context.Canceled
}

// isRetriable reports whether a build failure is worth another attempt. A build
// that ran to a deterministic failure (builder.ErrBuildFailed: a PKGBUILD or
// compile error, or no package produced) will fail identically on a retry, so it
// is terminal; every other failure (clone, image pull, network, a build timeout)
// is treated as transient and retried. Cancellation is handled by the caller and
// never classified here.
func isRetriable(err error) bool {
	if err == nil {
		return false
	}
	return !errors.Is(err, builder.ErrBuildFailed)
}
