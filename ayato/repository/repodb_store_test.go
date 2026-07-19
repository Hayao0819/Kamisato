package repository

import (
	"bytes"
	"fmt"
	"io"
	"path"
	"sort"
	"sync"

	"github.com/Hayao0819/Kamisato/ayato/repository/blob"
	"github.com/Hayao0819/Kamisato/ayato/stream"
)

type memStore struct {
	mu                sync.Mutex
	files             map[string][]byte
	versions          map[string]string
	nextVersion       int
	onFetch           func(name string)
	fetchErr          error
	storeErrName      string
	storeErr          error
	storeAfterErrName string
	storeAfterErr     error
	afterStoreName    string
	afterStore        func(name string)
}

func newMemStore() *memStore {
	return &memStore{
		files:    map[string][]byte{},
		versions: map[string]string{},
	}
}

func (store *memStore) key(repoName, arch, name string) string {
	return repoName + "/" + arch + "/" + name
}

func (store *memStore) put(repoName, arch, name string, body []byte) {
	key := store.key(repoName, arch, name)
	store.files[key] = body
	store.nextVersion++
	store.versions[key] = fmt.Sprintf("v%d", store.nextVersion)
}

func readAllSeek(file stream.SeekFile) ([]byte, error) {
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}
	return io.ReadAll(file)
}

func (store *memStore) StoreFile(repoName, arch string, file stream.SeekFile) error {
	body, err := readAllSeek(file)
	if err != nil {
		return err
	}
	store.mu.Lock()
	store.put(repoName, arch, path.Base(file.FileName()), body)
	store.mu.Unlock()
	return nil
}

func (store *memStore) FetchFileWithETag(
	repoName,
	arch,
	name string,
) (stream.File, string, error) {
	store.mu.Lock()
	key := store.key(repoName, arch, name)
	body, ok := store.files[key]
	etag := store.versions[key]
	hook := store.onFetch
	store.onFetch = nil
	fetchErr := store.fetchErr
	store.mu.Unlock()
	if hook != nil {
		hook(name)
	}
	if fetchErr != nil {
		return nil, "", fetchErr
	}
	if !ok {
		return nil, "", blob.ErrNotFound
	}
	return stream.NewFileStream(
		name,
		"application/octet-stream",
		nopSeekCloser{bytes.NewReader(body)},
	), etag, nil
}

func (store *memStore) StoreFileIfMatch(
	repoName,
	arch string,
	file stream.SeekFile,
	etag string,
) error {
	body, err := readAllSeek(file)
	if err != nil {
		return err
	}
	name := path.Base(file.FileName())
	store.mu.Lock()
	if name == store.storeErrName && store.storeErr != nil {
		store.mu.Unlock()
		return store.storeErr
	}
	key := store.key(repoName, arch, name)
	current, exists := store.versions[key]
	if etag == "" && exists || etag != "" && current != etag {
		store.mu.Unlock()
		return blob.ErrPreconditionFailed
	}
	store.put(repoName, arch, name, body)
	var afterErr error
	if name == store.storeAfterErrName {
		afterErr = store.storeAfterErr
	}
	hook := store.afterStore
	if name != store.afterStoreName {
		hook = nil
	} else {
		store.afterStore = nil
	}
	store.mu.Unlock()
	if hook != nil {
		hook(name)
	}
	return afterErr
}

func (*memStore) StoreFileWithSignedURL(string, string, string) (string, error) {
	return "", nil
}

func (store *memStore) DeleteFile(repoName, arch, name string) error {
	store.mu.Lock()
	delete(store.files, store.key(repoName, arch, name))
	store.mu.Unlock()
	return nil
}

func (store *memStore) FetchFile(repoName, arch, name string) (stream.File, error) {
	store.mu.Lock()
	body, ok := store.files[store.key(repoName, arch, name)]
	store.mu.Unlock()
	if !ok {
		return nil, blob.ErrNotFound
	}
	return stream.NewFileStream(
		name,
		"application/octet-stream",
		nopSeekCloser{bytes.NewReader(body)},
	), nil
}

func (*memStore) RepoNames() ([]string, error)           { return nil, nil }
func (*memStore) Files(string, string) ([]string, error) { return nil, nil }
func (*memStore) FilesWithMeta(string, string) ([]blob.FileInfo, error) {
	return nil, nil
}
func (*memStore) Arches(string) ([]string, error) { return nil, nil }

func (store *memStore) names(repoName, arch string) []string {
	store.mu.Lock()
	defer store.mu.Unlock()
	prefix := repoName + "/" + arch + "/"
	names := make([]string, 0)
	for key := range store.files {
		if len(key) > len(prefix) && key[:len(prefix)] == prefix {
			names = append(names, key[len(prefix):])
		}
	}
	sort.Strings(names)
	return names
}

type nopSeekCloser struct{ *bytes.Reader }

func (nopSeekCloser) Close() error { return nil }
