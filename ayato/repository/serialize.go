package repository

import (
	"sync"

	"github.com/Hayao0819/Kamisato/ayato/blob"
	"github.com/Hayao0819/Kamisato/ayato/stream"
)

type keyedMutex struct {
	mu    sync.Mutex
	locks map[string]*sync.Mutex
}

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
