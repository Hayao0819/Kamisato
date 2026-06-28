package service

import (
	"crypto/rand"
	"encoding/hex"
	"sort"
	"time"

	"github.com/Hayao0819/Kamisato/miko/domain"
)

func (s *Service) setStatus(id string, status domain.JobStatus, err error) {
	s.update(id, func(j *domain.BuildJob) {
		j.Status = status
		if err != nil {
			j.Err = err.Error()
		}
	})
}

func newJobID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		// Fall back to a time-based ID; collisions are improbable.
		return hex.EncodeToString([]byte(time.Now().Format("20060102150405.000000000")))
	}
	return hex.EncodeToString(b[:])
}

func (s *Service) update(id string, fn func(*domain.BuildJob)) {
	s.mu.Lock()
	job, ok := s.store[id]
	if ok {
		fn(job)
	}
	var snap *domain.BuildJob
	if ok {
		c := *job
		snap = &c
	}
	s.mu.Unlock()
	if snap != nil {
		s.persistSave(snap)
	}
}

// maxStoredJobs caps the in-memory job store to bound memory; older terminal
// jobs are dropped first (and removed from disk when persistence is on).
const maxStoredJobs = 500

// evictLocked drops the oldest terminal (success/failed) jobs until the store
// is within maxStoredJobs, returning the evicted IDs so the caller can remove
// them from disk. Queued/running jobs are never evicted. Callers must hold s.mu.
func (s *Service) evictLocked() []string {
	if len(s.store) <= maxStoredJobs {
		return nil
	}
	terminal := make([]*domain.BuildJob, 0, len(s.store))
	for _, j := range s.store {
		if j.Status == domain.JobStatusSuccess || j.Status == domain.JobStatusFailed {
			terminal = append(terminal, j)
		}
	}
	sort.Slice(terminal, func(i, j int) bool {
		return terminal[i].CreatedAt.Before(terminal[j].CreatedAt)
	})
	var evicted []string
	for _, j := range terminal {
		if len(s.store) <= maxStoredJobs {
			break
		}
		delete(s.store, j.ID)
		evicted = append(evicted, j.ID)
	}
	return evicted
}
