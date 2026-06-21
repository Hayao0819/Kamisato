package service

import (
	"context"
	"log/slog"

	"github.com/Hayao0819/Kamisato/miko/domain"
)

// defaultQueueSize is the buffer size of the in-memory job queue.
const defaultQueueSize = 128

// queue is an in-memory job queue backed by a buffered channel.
type queue struct {
	jobs chan *domain.BuildJob
}

func newQueue() *queue {
	return &queue{
		jobs: make(chan *domain.BuildJob, defaultQueueSize),
	}
}

// push enqueues a job. It returns an error if the queue is full.
func (q *queue) push(job *domain.BuildJob) error {
	select {
	case q.jobs <- job:
		return nil
	default:
		return ErrQueueFull
	}
}

// pop blocks until a job is available or the context is cancelled.
func (q *queue) pop(ctx context.Context) (*domain.BuildJob, bool) {
	select {
	case <-ctx.Done():
		return nil, false
	case job, ok := <-q.jobs:
		if !ok {
			return nil, false
		}
		slog.Debug("Popped job from queue", "id", job.ID)
		return job, true
	}
}
