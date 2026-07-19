package repository

import (
	"sync"
	"time"

	"github.com/Hayao0819/Kamisato/ayato/repository/blob"
	"github.com/Hayao0819/Kamisato/ayato/stream"
)

type keyedMutex struct {
	mu    sync.Mutex
	locks map[string]*sync.Mutex
}

// lock acquires the per-key mutex and returns its unlock func; callers use
// `defer k.lock(key)()` so it releases on every return path, even when the critical
// section blocks on blob.Store (S3) I/O.
//
// The acquire is deliberately NOT context-aware: ayato threads no request context
// into the repository/blob layer (the S3 backend runs on context.Background()), so
// there is no deadline to select on and a waiter blocks until the holder finishes.
// Making it cancellable would mean threading context.Context through blob.Store,
// BinaryRepository, and the service layer; the hold time is instead bounded by the
// S3 client's own timeouts.
func (k *keyedMutex) lock(key string) func() {
	k.mu.Lock()
	if k.locks == nil {
		k.locks = make(map[string]*sync.Mutex)
	}
	m, ok := k.locks[key]
	if !ok {
		m = &sync.Mutex{}
		k.locks[key] = m
	}
	k.mu.Unlock()

	m.Lock()
	return m.Unlock
}

type serializingStore struct {
	blob.Store
	mu keyedMutex
}

func newSerializingStore(s blob.Store) blob.Store {
	return &serializingStore{Store: s}
}

func (s *serializingStore) StoreFile(repo, arch string, file stream.SeekFile) error {
	defer s.mu.lock(repo + "/" + arch)()
	return s.Store.StoreFile(repo, arch, file)
}

func (s *serializingStore) DeleteFile(repo, arch, file string) error {
	defer s.mu.lock(repo + "/" + arch)()
	return s.Store.DeleteFile(repo, arch, file)
}

func (s *serializingStore) DeleteFileIfUnchanged(repo, arch string, expected blob.FileInfo, cutoff time.Time) (bool, error) {
	defer s.mu.lock(repo + "/" + arch)()
	return blob.DeleteOrphanIfUnchanged(s.Store, repo, arch, expected, cutoff)
}

func (s *serializingStore) LockPublication(repo string) (func(), error) {
	return blob.LockPublication(s.Store, repo)
}

// FetchFileWithMeta forwards the optional MetaFetcher capability through the
// wrapper. Embedding blob.Store promotes only the Store interface, so without
// this the type assertion in binaryRepository misses and conditional-GET
// validators silently degrade to a full body on every request. Reads are not
// serialized against writes (same as the embedded FetchFile).
func (s *serializingStore) FetchFileWithMeta(repo, arch, file string) (stream.File, blob.FileMeta, error) {
	return blob.FetchFileWithMeta(s.Store, repo, arch, file)
}
