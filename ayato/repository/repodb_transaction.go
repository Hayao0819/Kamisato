package repository

import (
	"fmt"
	"math/rand/v2"
	"path"
	"time"

	"github.com/Hayao0819/Kamisato/internal/errors"

	"github.com/Hayao0819/Kamisato/ayato/repository/blob"
	pacmanrepo "github.com/Hayao0819/Kamisato/pkg/pacman/repo"
)

// dbMutation owns the state that spans retries of one optimistic transaction.
// Package inputs stay in dir; only database artifacts are refreshed per attempt.
type dbMutation struct {
	repository         *binaryRepository
	repo               string
	arch               string
	dir                string
	dbPath             string
	useSignedDB        bool
	canonicalCommitted bool
}

func (r *binaryRepository) mutateDB(
	repo, arch, dir string,
	useSignedDB bool,
	mutate func(dbPath string) error,
) error {
	transaction := &dbMutation{
		repository:  r,
		repo:        repo,
		arch:        arch,
		dir:         dir,
		dbPath:      path.Join(dir, pacmanrepo.Artifacts(repo).DatabaseArchive()),
		useSignedDB: useSignedDB,
	}
	return transaction.run(mutate)
}

func (m *dbMutation) run(mutate func(dbPath string) error) error {
	var lastErr error
	for attempt := range maxDBAttempts {
		etags, err := m.prepare(mutate)
		if err != nil {
			return m.markCommitted(err)
		}
		err = m.repository.commitDB(m.repo, m.arch, m.dir, etags, m.useSignedDB)
		if err == nil {
			return nil
		}
		if CanonicalCommitted(err) {
			m.canonicalCommitted = true
		}
		if !errors.Is(err, blob.ErrPreconditionFailed) && !CanonicalCommitted(err) {
			return err
		}
		lastErr = err
		dbConflictBackoff(attempt)
	}
	return m.markCommitted(errors.WrapErr(
		lastErr,
		fmt.Sprintf(
			"repo db %s/%s: too many conflicting writers after %d attempts",
			m.repo,
			m.arch,
			maxDBAttempts,
		),
	))
}

func (m *dbMutation) prepare(mutate func(dbPath string) error) (map[string]string, error) {
	if err := clearDBArtifacts(m.dir, m.repo, m.useSignedDB); err != nil {
		return nil, err
	}
	etags, err := m.repository.fetchDBArtifacts(m.repo, m.arch, m.dir, m.useSignedDB)
	if err != nil {
		return nil, err
	}
	if err := mutate(m.dbPath); err != nil {
		return nil, err
	}
	return etags, nil
}

func (m *dbMutation) markCommitted(err error) error {
	if err == nil || !m.canonicalCommitted || CanonicalCommitted(err) {
		return err
	}
	return &CanonicalCommitError{Err: err}
}

func dbConflictBackoff(attempt int) {
	delay := dbConflictBackoffBase << attempt
	if delay > dbConflictBackoffCap {
		delay = dbConflictBackoffCap
	}
	time.Sleep(rand.N(delay)) //nolint:gosec // retry jitter does not require crypto randomness
}
