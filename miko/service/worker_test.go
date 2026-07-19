package service

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Hayao0819/Kamisato/internal/conf"
	"github.com/Hayao0819/Kamisato/miko/domain"
)

// A client-signed job's artifact dir must be swept once retention elapses even
// when no later job runs: the timer, not job completion, drives the sweep.
func TestSweepLoopReclaimsIdleArtifacts(t *testing.T) {
	s := New(&conf.MikoConfig{})

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
	s := New(&conf.MikoConfig{})

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
	s := New(&conf.MikoConfig{Concurrency: 3})
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
