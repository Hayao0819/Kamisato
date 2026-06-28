package service

import (
	"context"
	"log/slog"

	"github.com/Hayao0819/Kamisato/miko/domain"
)

const defaultQueueSize = 128

type queue struct {
	jobs chan *domain.BuildJob
}

func newQueue() *queue {
	return &queue{
		jobs: make(chan *domain.BuildJob, defaultQueueSize),
	}
}

func (q *queue) push(job *domain.BuildJob) error {
	select {
	case q.jobs <- job:
		return nil
	default:
		return ErrQueueFull
	}
}

func (q *queue) len() int {
	return len(q.jobs)
}

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
