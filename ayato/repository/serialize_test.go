package repository

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/Hayao0819/Kamisato/ayato/stream"
)

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

func (p *concurrencyProbe) RepoAdd(_, _ string, _, _ stream.SeekFile, _ bool, _ *string) error {
	p.enter()
	time.Sleep(2 * time.Millisecond)
	p.leave()
	return nil
}

func (p *concurrencyProbe) StoreFile(string, string, stream.SeekFile) error { return nil }
func (p *concurrencyProbe) StoreFileWithSignedURL(string, string, string) (string, error) {
	return "", nil
}
func (p *concurrencyProbe) DeleteFile(string, string, string) error               { return nil }
func (p *concurrencyProbe) FetchFile(string, string, string) (stream.File, error) { return nil, nil }
func (p *concurrencyProbe) RepoRemove(string, string, string, bool, *string) error {
	return nil
}
func (p *concurrencyProbe) InitArch(string, string, bool, *string) error { return nil }
func (p *concurrencyProbe) RepoNames() ([]string, error)                 { return nil, nil }
func (p *concurrencyProbe) Files(string, string) ([]string, error)       { return nil, nil }
func (p *concurrencyProbe) Arches(string) ([]string, error)              { return nil, nil }

func runConcurrent(s Store, repo string, arches []string) {
	start := make(chan struct{})
	var wg sync.WaitGroup
	for _, arch := range arches {
		wg.Add(1)
		go func(arch string) {
			defer wg.Done()
			<-start
			_ = s.RepoAdd(repo, arch, nil, nil, false, nil)
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
	runConcurrent(s, "myrepo", arches)

	if probe.maxSeen != 1 {
		t.Errorf("same (repo,arch) RepoAdd overlapped: peak inflight = %d, want 1", probe.maxSeen)
	}
}

func TestSerializingStoreDifferentKeys(t *testing.T) {
	probe := &concurrencyProbe{}
	s := newSerializingStore(probe)

	arches := make([]string, 8)
	for i := range arches {
		arches[i] = fmt.Sprintf("arch%d", i)
	}
	runConcurrent(s, "myrepo", arches)

	if probe.maxSeen < 2 {
		t.Errorf("different (repo,arch) RepoAdd did not run in parallel: peak inflight = %d, want >= 2", probe.maxSeen)
	}
}
