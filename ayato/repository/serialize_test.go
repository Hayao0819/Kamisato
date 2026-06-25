package repository

import (
	"fmt"
	"os/exec"
	"sync"
	"testing"
	"time"

	"github.com/Hayao0819/Kamisato/ayato/blob"
	"github.com/Hayao0819/Kamisato/ayato/stream"
)

// concurrencyProbe is a blob.Store whose StoreFile records peak in-flight calls,
// so a test can assert whether overlapping calls were serialized.
type concurrencyProbe struct {
	mu       sync.Mutex
	inflight int
	maxSeen  int
}

func (p *concurrencyProbe) enter() {
	p.mu.Lock()
	p.inflight++
	if p.inflight > p.maxSeen {
		p.maxSeen = p.inflight
	}
	p.mu.Unlock()
}

func (p *concurrencyProbe) leave() {
	p.mu.Lock()
	p.inflight--
	p.mu.Unlock()
}

func (p *concurrencyProbe) StoreFile(string, string, stream.SeekFile) error {
	p.enter()
	time.Sleep(2 * time.Millisecond)
	p.leave()
	return nil
}

func (p *concurrencyProbe) StoreFileWithSignedURL(string, string, string) (string, error) {
	return "", nil
}
func (p *concurrencyProbe) DeleteFile(string, string, string) error               { return nil }
func (p *concurrencyProbe) FetchFile(string, string, string) (stream.File, error) { return nil, nil }
func (p *concurrencyProbe) RepoNames() ([]string, error)                          { return nil, nil }
func (p *concurrencyProbe) Files(string, string) ([]string, error)                { return nil, nil }
func (p *concurrencyProbe) Arches(string) ([]string, error)                       { return nil, nil }

func runConcurrentStore(s blob.Store, repo string, arches []string) {
	start := make(chan struct{})
	var wg sync.WaitGroup
	for _, arch := range arches {
		wg.Add(1)
		go func(arch string) {
			defer wg.Done()
			<-start
			_ = s.StoreFile(repo, arch, nil)
		}(arch)
	}
	close(start)
	wg.Wait()
}

func TestSerializingStoreSameKey(t *testing.T) {
	probe := &concurrencyProbe{}
	s := newSerializingStore(probe)

	arches := make([]string, 20)
	for i := range arches {
		arches[i] = "x86_64"
	}
	runConcurrentStore(s, "myrepo", arches)

	if probe.maxSeen != 1 {
		t.Errorf("same (repo,arch) StoreFile overlapped: peak inflight = %d, want 1", probe.maxSeen)
	}
}

func TestSerializingStoreDifferentKeys(t *testing.T) {
	probe := &concurrencyProbe{}
	s := newSerializingStore(probe)

	arches := make([]string, 8)
	for i := range arches {
		arches[i] = fmt.Sprintf("arch%d", i)
	}
	runConcurrentStore(s, "myrepo", arches)

	if probe.maxSeen < 2 {
		t.Errorf("different (repo,arch) StoreFile did not run in parallel: peak inflight = %d, want >= 2", probe.maxSeen)
	}
}

// repoAddProbe is a blob.Store whose StoreFile records peak in-flight calls, used
// to observe whether binaryRepository.RepoAdd serializes its DB read-modify-write
// per (repo, arch). FetchFile returns os.ErrNotExist (no existing DB) so RepoAdd
// takes the fresh-repo path without invoking repo-add against missing artifacts.
type repoAddProbe struct {
	concurrencyProbe
}

func (p *repoAddProbe) FetchFile(string, string, string) (stream.File, error) {
	return nil, errProbeMiss
}

var errProbeMiss = fmt.Errorf("not found")

// runConcurrentRepoAdd fires N RepoAdds with a nil package (so repo-add is a
// no-op create on a fresh temp DB) and lets storeArtifacts hit the probe.
func runConcurrentRepoAdd(r *binaryRepository, repo string, arches []string) {
	start := make(chan struct{})
	var wg sync.WaitGroup
	for _, arch := range arches {
		wg.Add(1)
		go func(arch string) {
			defer wg.Done()
			<-start
			_ = r.RepoAdd(repo, arch, nil, nil, false, nil)
		}(arch)
	}
	close(start)
	wg.Wait()
}

// TestRepoAddSerializedNoDeadlock proves two things at once: binaryRepository's
// own dbMu serializes RepoAdd per (repo, arch), AND wrapping the blob.Store in a
// serializingStore (whose StoreFile takes its OWN keyed mutex on the same key)
// does not deadlock — the two keyed mutexes are distinct instances, so the
// nested StoreFile lock is never re-entrant. Run under -race.
func TestRepoAddSerializedNoDeadlock(t *testing.T) {
	if _, err := exec.LookPath("repo-add"); err != nil {
		t.Skip("repo-add not installed; skipping RepoAdd concurrency test")
	}
	probe := &repoAddProbe{}
	r := &binaryRepository{Store: newSerializingStore(probe)}

	arches := make([]string, 16)
	for i := range arches {
		arches[i] = "x86_64"
	}

	done := make(chan struct{})
	go func() {
		runConcurrentRepoAdd(r, "myrepo", arches)
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("RepoAdd deadlocked (nested keyed mutexes)")
	}

	if probe.maxSeen != 1 {
		t.Errorf("same (repo,arch) RepoAdd overlapped: peak inflight = %d, want 1", probe.maxSeen)
	}
}
