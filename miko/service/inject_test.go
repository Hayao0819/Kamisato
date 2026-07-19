package service

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/Hayao0819/Kamisato/internal/errors"

	"github.com/Hayao0819/Kamisato/internal/conf"
	"github.com/Hayao0819/Kamisato/miko/domain"
)

type failingSigner struct{}

func (failingSigner) Sign(context.Context, string) (string, error) {
	return "", errors.New("signer unavailable")
}

type failSecondSigner struct{ calls int }

func (s *failSecondSigner) Sign(_ context.Context, path string) (string, error) {
	s.calls++
	if s.calls == 2 {
		return "", errors.New("second signature failed")
	}
	signature := path + ".sig"
	if err := os.WriteFile(signature, []byte("signature"), 0o600); err != nil {
		return "", err
	}
	return signature, nil
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
	calls    int
	repo     string
	uploaded []PackageUpload
}

func (f *fakeUploader) Upload(_ context.Context, repo string, packages []PackageUpload) error {
	f.calls++
	f.repo = repo
	f.uploaded = append(f.uploaded, packages...)
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

	pkgPath := filepath.Join(t.TempDir(), "foo.pkg.tar.zst")
	if err := os.WriteFile(pkgPath, []byte("pkg"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := s.signAndUpload(context.Background(), "extra", []string{pkgPath}); err != nil {
		t.Fatalf("signAndUpload: %v", err)
	}
	if fu.calls != 1 || fu.repo != "extra" || len(fu.uploaded) != 1 || fu.uploaded[0] != (PackageUpload{PackagePath: pkgPath}) {
		t.Errorf("uploader received %v, want one unsigned upload to extra", fu.uploaded)
	}
}

func TestSignAndUploadPublishesSplitPackagesInOneBatch(t *testing.T) {
	fu := &fakeUploader{}
	s := New(&conf.MikoConfig{}, nil, nil, fu).(*Service)
	dir := t.TempDir()
	paths := []string{filepath.Join(dir, "foo.pkg.tar.zst"), filepath.Join(dir, "foo-docs.pkg.tar.zst")}
	for _, path := range paths {
		if err := os.WriteFile(path, []byte("pkg"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.signAndUpload(context.Background(), "extra", paths); err != nil {
		t.Fatal(err)
	}
	if fu.calls != 1 || len(fu.uploaded) != 2 {
		t.Fatalf("batch calls = %d, packages = %d", fu.calls, len(fu.uploaded))
	}
}

// A signer failure must fail the publish closed: no package is uploaded unsigned.
func TestSignAndUploadFailsClosedOnSignerError(t *testing.T) {
	fu := &fakeUploader{}
	s := New(&conf.MikoConfig{}, failingSigner{}, nil, fu).(*Service)

	pkgPath := filepath.Join(t.TempDir(), "foo.pkg.tar.zst")
	if err := os.WriteFile(pkgPath, []byte("pkg"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := s.signAndUpload(context.Background(), "extra", []string{pkgPath}); err == nil {
		t.Fatal("signAndUpload must fail when the signer errors")
	}
	if len(fu.uploaded) != 0 {
		t.Fatalf("nothing must be uploaded on a signer error, got %v", fu.uploaded)
	}
}

func TestSignAndUploadDoesNotPublishWhenLaterSignatureFails(t *testing.T) {
	fu := &fakeUploader{}
	s := New(&conf.MikoConfig{}, &failSecondSigner{}, nil, fu).(*Service)
	dir := t.TempDir()
	var packages []string
	for _, name := range []string{"foo.pkg.tar.zst", "foo-docs.pkg.tar.zst"} {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte("pkg"), 0o600); err != nil {
			t.Fatal(err)
		}
		packages = append(packages, path)
	}
	if err := s.signAndUpload(context.Background(), "extra", packages); err == nil {
		t.Fatal("later signer failure unexpectedly succeeded")
	}
	if fu.calls != 0 {
		t.Fatalf("partial batch was uploaded after later signer failure: %#v", fu.uploaded)
	}
}

func TestSignAndUploadRejectsPackageOverMaxSize(t *testing.T) {
	fu := &fakeUploader{}
	s := New(&conf.MikoConfig{MaxSize: 2}, nil, nil, fu).(*Service)
	dir := t.TempDir()
	smallPath := filepath.Join(dir, "small.pkg.tar.zst")
	if err := os.WriteFile(smallPath, []byte("ok"), 0o600); err != nil {
		t.Fatal(err)
	}
	pkgPath := filepath.Join(dir, "large.pkg.tar.zst")
	if err := os.WriteFile(pkgPath, []byte("three"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := s.signAndUpload(context.Background(), "extra", []string{smallPath, pkgPath}); err == nil {
		t.Fatal("package larger than max_size must be rejected")
	}
	if len(fu.uploaded) != 0 {
		t.Fatalf("oversized package was uploaded: %v", fu.uploaded)
	}
}
