package service

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/Hayao0819/Kamisato/internal/errors"
	"github.com/Hayao0819/Kamisato/miko/domain"
	"github.com/Hayao0819/Kamisato/pkg/atomicfile"
)

// Persister durably stores and reloads job records. The service depends on this
// seam rather than a concrete store so a fake can stand in for tests; jobPersist
// is the production implementation.
type Persister interface {
	save(job *domain.BuildJob) error
	remove(id string) error
	loadAll() ([]*domain.BuildJob, error)
}

var _ Persister = (*jobPersist)(nil)

// jobPersist stores each job as one JSON file under <dataDir>/jobs. BuildJob's
// Request is json:"-", so only durable state is written.
type jobPersist struct {
	dir string
}

func newJobPersist(dataDir string) (*jobPersist, error) {
	dir := filepath.Join(dataDir, "jobs")
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return nil, err
	}
	return &jobPersist{dir: dir}, nil
}

// NewFilePersister returns the file-backed Persister injected into the service in
// production. Callers pass its result to New; a nil Persister disables persistence.
func NewFilePersister(dataDir string) (Persister, error) {
	p, err := newJobPersist(dataDir)
	if err != nil {
		return nil, err
	}
	return p, nil
}

func (p *jobPersist) path(id string) string {
	return filepath.Join(p.dir, id+".json")
}

// save writes job atomically (temp file + rename). Callers must pass a snapshot,
// not a job the worker may mutate concurrently.
func (p *jobPersist) save(job *domain.BuildJob) error {
	data, err := json.MarshalIndent(job, "", "  ")
	if err != nil {
		return err
	}
	return atomicfile.WriteFile(p.path(job.ID), data, 0o600)
}

func (p *jobPersist) remove(id string) error {
	if err := atomicfile.Remove(p.path(id)); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

// loadAll reads every persisted job. Unreadable or malformed files are skipped.
func (p *jobPersist) loadAll() ([]*domain.BuildJob, error) {
	entries, err := os.ReadDir(p.dir)
	if err != nil {
		return nil, err
	}
	jobs := make([]*domain.BuildJob, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(p.dir, e.Name()))
		if err != nil {
			continue
		}
		var job domain.BuildJob
		if err := json.Unmarshal(data, &job); err != nil {
			continue
		}
		jobs = append(jobs, &job)
	}
	return jobs, nil
}
