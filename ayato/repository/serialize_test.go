package repository

import (
	"fmt"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/Hayao0819/Kamisato/ayato/repository/blob"
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

func (p *concurrencyProbe) FetchFileWithETag(string, string, string) (stream.File, string, error) {
	return nil, "", nil
}

func (p *concurrencyProbe) StoreFileIfMatch(string, string, stream.SeekFile, string) error {
	p.enter()
	time.Sleep(2 * time.Millisecond)
	p.leave()
	return nil
}

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

// TestSerializingStoreSameKey stays on the real scheduler: it relies on genuine
// sync.Mutex contention on one key, and a goroutine blocked on Mutex.Lock is not
// "durably blocked" under synctest, so the fake clock could never advance past the
// holder's sleep. The assertion is deterministic regardless of timing — the store
// serializes same-key writes, so peak in-flight must be exactly 1.
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
	synctest.Test(t, func(t *testing.T) {
		probe := &concurrencyProbe{}
		s := newSerializingStore(probe)

		arches := make([]string, 8)
		for i := range arches {
			arches[i] = fmt.Sprintf("arch%d", i)
		}

		start := make(chan struct{})
		var wg sync.WaitGroup
		for _, arch := range arches {
			wg.Add(1)
			go func(arch string) {
				defer wg.Done()
				<-start
				_ = s.StoreFile("myrepo", arch, nil)
			}(arch)
		}
		close(start)

		// Distinct keys mean uncontended mutexes, so every goroutine reaches the
		// probe's sleep and durably blocks; once Wait returns, peak in-flight is
		// deterministic rather than a scheduling race.
		synctest.Wait()
		if probe.maxSeen != len(arches) {
			t.Errorf("different (repo,arch) StoreFile did not run in parallel: peak inflight = %d, want %d", probe.maxSeen, len(arches))
		}

		wg.Wait()
	})
}

// repoAddProbe is a blob.Store whose StoreFile records peak in-flight calls, used
// to observe whether binaryRepository.RepoAdd serializes its DB read-modify-write
// per (repo, arch). FetchFile returns a miss (no existing DB) so RepoAdd takes
// the fresh-repo path, building an empty database with the native writer.
type repoAddProbe struct {
	concurrencyProbe
}

func (p *repoAddProbe) FetchFile(string, string, string) (stream.File, error) {
	return nil, errProbeMiss
}

func (p *repoAddProbe) FetchFileWithETag(string, string, string) (stream.File, string, error) {
	return nil, "", errProbeMiss
}

var errProbeMiss = blob.ErrNotFound

// runConcurrentRepoAdd fires N RepoAdds with a nil package (so the native writer
// just emits an empty DB into a fresh temp dir) and lets storeArtifacts hit the
// probe.
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
