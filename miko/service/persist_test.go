package service

import (
	"testing"
	"time"

	"github.com/Hayao0819/Kamisato/internal/conf"
	"github.com/Hayao0819/Kamisato/miko/domain"
)

func TestJobPersistRoundtrip(t *testing.T) {
	p, err := newJobPersist(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	job := &domain.BuildJob{
		ID:        "abc123",
		Repo:      "r",
		Arch:      "x86_64",
		Status:    domain.JobStatusSuccess,
		Logs:      "built ok",
		Packages:  []string{"foo-1.0-1-x86_64.pkg.tar.zst"},
		CreatedAt: time.Now(),
	}
	if err := p.save(job); err != nil {
		t.Fatal(err)
	}

	got, err := p.loadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("want 1 job, got %d", len(got))
	}
	if got[0].ID != "abc123" || got[0].Logs != "built ok" || got[0].Status != domain.JobStatusSuccess {
		t.Errorf("roundtrip mismatch: %+v", got[0])
	}

	if err := p.remove("abc123"); err != nil {
		t.Fatal(err)
	}
	if got, _ := p.loadAll(); len(got) != 0 {
		t.Errorf("want 0 after remove, got %d", len(got))
	}
}

func TestRestoreMarksInterrupted(t *testing.T) {
	dir := t.TempDir()
	p, err := newJobPersist(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := p.save(&domain.BuildJob{ID: "run1", Status: domain.JobStatusRunning, CreatedAt: time.Now()}); err != nil {
		t.Fatal(err)
	}
	if err := p.save(&domain.BuildJob{ID: "done1", Status: domain.JobStatusSuccess, CreatedAt: time.Now()}); err != nil {
		t.Fatal(err)
	}

	s := New(&conf.MikoConfig{DataDir: dir}, WithPersister(p))

	run, err := s.Status("run1")
	if err != nil {
		t.Fatal(err)
	}
	if run.Status != domain.JobStatusFailed || run.Err == "" {
		t.Errorf("interrupted job not marked failed: %+v", run)
	}
	done, err := s.Status("done1")
	if err != nil {
		t.Fatal(err)
	}
	if done.Status != domain.JobStatusSuccess {
		t.Errorf("terminal job changed: %+v", done)
	}
}
