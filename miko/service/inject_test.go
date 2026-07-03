package service

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/Hayao0819/Kamisato/internal/conf"
	"github.com/Hayao0819/Kamisato/miko/domain"
)

type failingSigner struct{}

func (failingSigner) Sign(context.Context, string) (string, error) {
	return "", errors.New("signer unavailable")
}

type fakePersister struct {
	mu    sync.Mutex
	saved map[string]*domain.BuildJob
}

func (f *fakePersister) save(job *domain.BuildJob) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.saved == nil {
		f.saved = map[string]*domain.BuildJob{}
	}
	f.saved[job.ID] = job
	return nil
}

func (f *fakePersister) remove(id string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.saved, id)
	return nil
}

func (f *fakePersister) loadAll() ([]*domain.BuildJob, error) { return nil, nil }

type fakeUploader struct {
	uploaded [][3]string
}

func (f *fakeUploader) Upload(repo, pkgPath, sigPath string) error {
	f.uploaded = append(f.uploaded, [3]string{repo, pkgPath, sigPath})
	return nil
}

// Submit must persist through whatever Persister is injected, not a store the
// service builds itself.
func TestSubmitPersistsThroughInjectedPersister(t *testing.T) {
	fp := &fakePersister{}
	s := New(&conf.MikoConfig{}, nil, fp, nil)

	id, err := s.Submit(&domain.BuildRequest{Arch: "x86_64", Pkgbuild: "pkgname=foo"})
	if err != nil {
		t.Fatalf("submit: %v", err)
	}

	fp.mu.Lock()
	_, ok := fp.saved[id]
	fp.mu.Unlock()
	if !ok {
		t.Errorf("job %s was not persisted through the injected Persister", id)
	}
}

// signAndUpload must publish through the injected Uploader.
func TestSignAndUploadUsesInjectedUploader(t *testing.T) {
	fu := &fakeUploader{}
	s := New(&conf.MikoConfig{}, nil, nil, fu).(*Service)

	if err := s.signAndUpload(context.Background(), "extra", []string{"/tmp/foo.pkg.tar.zst"}); err != nil {
		t.Fatalf("signAndUpload: %v", err)
	}
	if len(fu.uploaded) != 1 || fu.uploaded[0] != [3]string{"extra", "/tmp/foo.pkg.tar.zst", ""} {
		t.Errorf("uploader received %v, want one unsigned upload to extra", fu.uploaded)
	}
}

// A signer failure must fail the publish closed: no package is uploaded unsigned.
func TestSignAndUploadFailsClosedOnSignerError(t *testing.T) {
	fu := &fakeUploader{}
	s := New(&conf.MikoConfig{}, failingSigner{}, nil, fu).(*Service)

	if err := s.signAndUpload(context.Background(), "extra", []string{"/tmp/foo.pkg.tar.zst"}); err == nil {
		t.Fatal("signAndUpload must fail when the signer errors")
	}
	if len(fu.uploaded) != 0 {
		t.Fatalf("nothing must be uploaded on a signer error, got %v", fu.uploaded)
	}
}
