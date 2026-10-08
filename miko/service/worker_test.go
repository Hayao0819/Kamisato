package service

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Hayao0819/Kamisato/miko/domain"
)

// A client-signed job's artifact dir must be swept once retention elapses even
// when no later job runs: the timer, not job completion, drives the sweep.
func TestSweepLoopReclaimsIdleArtifacts(t *testing.T) {
	s := New(Settings{})

	artDir := filepath.Join(t.TempDir(), "out")
	if err := os.MkdirAll(artDir, 0o755); err != nil {
		t.Fatal(err)
	}

	ended := time.Now().Add(-time.Hour)
	s.mu.Lock()
	s.store["job1"] = &domain.BuildJob{
		ID:          "job1",
		Status:      domain.JobStatusSuccess,
		Request:     &domain.BuildRequest{SignMode: domain.SignClient},
		ArtifactDir: artDir,
		EndedAt:     &ended,
	}
	s.mu.Unlock()

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		s.sweepLoop(ctx, time.Millisecond, time.Millisecond)
	}()
	defer func() {
		cancel()
		<-done
	}()

	deadline := time.After(2 * time.Second)
	for {
		if _, err := os.Stat(artDir); os.IsNotExist(err) {
			break
		}
		select {
		case <-deadline:
			t.Fatal("artifact dir not swept by the timer")
		case <-time.After(2 * time.Millisecond):
		}
	}

	s.mu.Lock()
	got := s.store["job1"].ArtifactDir
	s.mu.Unlock()
	if got != "" {
		t.Errorf("ArtifactDir not cleared after sweep: %q", got)
	}
}

// sweepLoop must exit promptly when its context is cancelled, leaking no
// goroutine and stopping its ticker.
func TestSweepLoopStopsOnContextCancel(t *testing.T) {
	s := New(Settings{})

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		s.sweepLoop(ctx, time.Hour, time.Hour)
	}()

	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("sweepLoop did not stop after context cancel")
	}
}

func TestRunOwnsWorkersAndStopsOnContextCancel(t *testing.T) {
	s := New(Settings{Workers: 3})
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		s.Run(ctx)
	}()

	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not wait for and stop its worker set")
	}
}

func TestCancelledJobsAreEvictedWithoutResurrectingQueuedBuilds(t *testing.T) {
	s := New(Settings{})
	oldest := &domain.BuildJob{ID: "cancelled-000", Status: domain.JobStatusCancelled}
	s.newLogBuffer(oldest.ID)
	s.mu.Lock()
	for i := range maxStoredJobs {
		job := &domain.BuildJob{
			ID: fmt.Sprintf("cancelled-%03d", i), Status: domain.JobStatusCancelled,
			CreatedAt: time.Unix(int64(i), 0),
		}
		s.store[job.ID] = job
	}
	s.store["queued"] = &domain.BuildJob{ID: "queued", Status: domain.JobStatusQueued}
	evicted := s.evictLocked()
	s.mu.Unlock()
	if len(evicted) != 1 || evicted[0] != oldest.ID || len(s.List()) != maxStoredJobs {
		t.Fatalf("evicted = %v, remaining = %d", evicted, len(s.List()))
	}
	if _, err := s.Status("queued"); err != nil {
		t.Fatal("non-terminal queued job was evicted")
	}
	// Popping the queue's stale pointer must stop before source/backend access.
	// A nil Request deliberately makes any accidental execution fail the test.
	s.process(t.Context(), oldest)
	if s.LogBuffer(oldest.ID) != nil {
		t.Fatal("evicted job retained its live log buffer")
	}
	if _, err := s.Status(oldest.ID); err == nil {
		t.Fatal("evicted job was resurrected")
	}
}
