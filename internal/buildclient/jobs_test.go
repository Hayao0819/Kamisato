package buildclient

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestWaitJobCancelsOnContext(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(Job{ID: "job-1", Status: "queued"})
	}))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	start := time.Now()
	_, err := WaitJob(ctx, srv.URL, "", "job-1", nil)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("WaitJob err = %v, want context.DeadlineExceeded", err)
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Fatalf("WaitJob took %v on a queued job, expected prompt cancellation", elapsed)
	}
}
