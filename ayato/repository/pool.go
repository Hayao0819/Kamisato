package repository

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"path"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/Hayao0819/Kamisato/internal/errors"

	"github.com/Hayao0819/Kamisato/ayato/repository/blob"
	"github.com/Hayao0819/Kamisato/ayato/repository/kv"
	"github.com/Hayao0819/Kamisato/ayato/stream"
)

// The pool stores package bytes content-addressed so identical content is held once
// and old versions can be retained/collected. The reserved (repo, arch) is registered
// with the backend by initBinaryStore and filtered out of RepoNames so it is never a
// servable pacman repo.
const (
	poolRepo = "_pool_"
	poolArch = "objects"

	// poolPtrNS maps a repo path (repo/arch/filename) to the hash of the pool
	// object holding its bytes; poolObjNS holds one manifest per pool object.
	poolPtrNS = "poolptr"
	poolObjNS = "poolobj"
)

// pkgNameRe extracts the pkgname (the retention group) from a pacman package file
// name: <pkgname>-<version>-<pkgrel>-<arch>.pkg.tar.<ext>, optionally with a
// trailing .sig. A name that does not match is its own group.
var pkgNameRe = regexp.MustCompile(`^(.+)-[^-]+-[^-]+-[^-]+\.pkg\.tar\.[^.]+(?:\.sig)?$`)

// poolObject is the per-hash manifest. UnrefAt is the time the GC first observed
// the object unreferenced (0 while referenced), so a grace window is measured from
// when the last pointer dropped rather than from upload.
type poolObject struct {
	Filename  string `json:"f"`
	Size      int64  `json:"s"`
	CreatedAt int64  `json:"c"`
	UnrefAt   int64  `json:"u,omitempty"`
}

// PoolPolicy governs the retention GC. An unreferenced object is deleted only once
// it has been unreferenced for at least RetentionWindow AND it is not among the
// newest KeepUnreferenced versions of its group; a referenced object is always
// kept. Conservative by construction: when in doubt it keeps.
type PoolPolicy struct {
	KeepUnreferenced int
	RetentionWindow  time.Duration
}

// PoolGCResult reports what a CollectPool run did.
type PoolGCResult struct {
	Scanned        int
	Deleted        int
	KeptReferenced int
	KeptRetained   int
}

// PoolCollector runs the retention GC. The factory returns one (nil when the pool
// is disabled) so the service layer can expose CollectPool.
type PoolCollector interface {
	CollectPool(ctx context.Context, policy PoolPolicy) (PoolGCResult, error)
}

// poolStore decorates a blob.Store with a content-addressed object pool: package
// bytes go to pool/<sha256> once and the (repo, arch, filename) path becomes a kv
// pointer to it, so identical content shares one object. Non-package files (db
// archives, signatures) and legacy pointer-less packages pass through unchanged, so
// already-published repos keep working.
type poolStore struct {
	blob.Store
	kv     kv.Store
	now    func() time.Time
	hashMu keyedMutex
}

var (
	_ blob.Store       = (*poolStore)(nil)
	_ blob.MetaFetcher = (*poolStore)(nil)
	_ PoolCollector    = (*poolStore)(nil)
)

func newPoolStore(under blob.Store, kvStore kv.Store) *poolStore {
	return &poolStore{Store: under, kv: kvStore, now: time.Now}
}

// isPoolable reports whether a file name is a pacman package (or its signature), the
// only content the pool holds; repo-DB archives never match and pass through unpooled.
func isPoolable(name string) bool {
	return strings.Contains(name, ".pkg.tar.")
}

func ptrKey(repo, arch, name string) string { return repo + "/" + arch + "/" + name }

// pointer returns the pool hash for a repo path, or "" when none exists (a db
// artifact or a legacy directly-stored file).
func (p *poolStore) pointer(repo, arch, name string) (string, error) {
	v, err := p.kv.Get(poolPtrNS, ptrKey(repo, arch, name))
	if errors.Is(err, kv.ErrNotFound) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return string(v), nil
}

// StoreFile content-addresses a package file into the pool and writes a pointer;
// non-package files pass straight through to the underlying store.
func (p *poolStore) StoreFile(repo, arch string, file stream.SeekFile) error {
	name := path.Base(file.FileName())
	if repo == poolRepo || !isPoolable(name) {
		return p.Store.StoreFile(repo, arch, file)
	}

	hash, size, err := hashSeek(file)
	if err != nil {
		return err
	}
	if err := p.ensureObject(hash, size, name, file); err != nil {
		return err
	}
	// The pointer is durable metadata (ttl 0), removed by DeleteFile/RepoRemove.
	return p.kv.Set(poolPtrNS, ptrKey(repo, arch, name), []byte(hash), 0)
}

// ensureObject writes the pool bytes and manifest for hash if absent. The per-hash
// lock serializes writers of the same content so a reader never sees a half-written
// object; an already-present object is a no-op (the dedup fast path).
func (p *poolStore) ensureObject(hash string, size int64, name string, file stream.SeekFile) error {
	defer p.hashMu.lock(hash)()

	if _, err := p.kv.Get(poolObjNS, hash); err == nil {
		return nil // already pooled
	} else if !errors.Is(err, kv.ErrNotFound) {
		return err
	}

	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return errors.WrapErr(err, "pool: seek package")
	}
	named := stream.NewFileStream(hash, file.ContentType(), file)
	if err := p.Store.StoreFile(poolRepo, poolArch, named); err != nil {
		return errors.WrapErr(err, "pool: store object")
	}
	obj := poolObject{Filename: name, Size: size, CreatedAt: p.now().UnixNano()}
	b, _ := json.Marshal(obj)
	return p.kv.Set(poolObjNS, hash, b, 0)
}

// FetchFile resolves a pooled package through its pointer, else serves the
// underlying object directly (a db artifact or a legacy directly-stored package).
func (p *poolStore) FetchFile(repo, arch, name string) (stream.File, error) {
	hash, err := p.pointer(repo, arch, name)
	if err != nil {
		return nil, err
	}
	if hash == "" {
		return p.Store.FetchFile(repo, arch, name)
	}
	f, ferr := p.Store.FetchFile(poolRepo, poolArch, hash)
	if ferr != nil {
		return nil, ferr
	}
	return aliasFile{File: f, name: name}, nil
}

func (p *poolStore) FetchFileWithETag(repo, arch, name string) (stream.File, string, error) {
	hash, err := p.pointer(repo, arch, name)
	if err != nil {
		return nil, "", err
	}
	if hash == "" {
		return p.Store.FetchFileWithETag(repo, arch, name)
	}
	f, etag, ferr := p.Store.FetchFileWithETag(poolRepo, poolArch, hash)
	if ferr != nil {
		return nil, "", ferr
	}
	return aliasFile{File: f, name: name}, etag, nil
}

// FetchFileWithMeta mirrors FetchFile while preserving the backend's conditional-GET
// validators (they belong to the pool object). A backend without metadata support
// degrades to a validator-free body, like the underlying store.
func (p *poolStore) FetchFileWithMeta(repo, arch, name string) (stream.File, blob.FileMeta, error) {
	mf, ok := p.Store.(blob.MetaFetcher)
	if !ok {
		f, err := p.FetchFile(repo, arch, name)
		return f, blob.FileMeta{}, err
	}
	hash, err := p.pointer(repo, arch, name)
	if err != nil {
		return nil, blob.FileMeta{}, err
	}
	if hash == "" {
		return mf.FetchFileWithMeta(repo, arch, name)
	}
	f, meta, ferr := mf.FetchFileWithMeta(poolRepo, poolArch, hash)
	if ferr != nil {
		return nil, blob.FileMeta{}, ferr
	}
	return aliasFile{File: f, name: name}, meta, nil
}

// StoreFileWithSignedURL presigns the pool object a package points at, so a
// redirect download hits the real bytes; non-pooled names presign as before.
func (p *poolStore) StoreFileWithSignedURL(repo, arch, name string) (string, error) {
	hash, err := p.pointer(repo, arch, name)
	if err != nil {
		return "", err
	}
	if hash == "" {
		return p.Store.StoreFileWithSignedURL(repo, arch, name)
	}
	return p.Store.StoreFileWithSignedURL(poolRepo, poolArch, hash)
}

// DeleteFile drops a pooled package's pointer (leaving the shared bytes for the GC
// to reclaim once no pointer remains); a non-pooled name deletes directly.
func (p *poolStore) DeleteFile(repo, arch, name string) error {
	hash, err := p.pointer(repo, arch, name)
	if err != nil {
		return err
	}
	if hash == "" {
		return p.Store.DeleteFile(repo, arch, name)
	}
	return p.kv.Delete(poolPtrNS, ptrKey(repo, arch, name))
}

// Files merges the underlying directory listing (db artifacts, legacy files) with
// the pooled package names pointed at (repo, arch), so listings still show every
// package even though its bytes live in the pool.
func (p *poolStore) Files(repo, arch string) ([]string, error) {
	if repo == poolRepo {
		return []string{}, nil
	}
	under, err := p.Store.Files(repo, arch)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	// A missing underlying directory just means every file for (repo, arch) is
	// pooled (its db artifacts not yet written); the pointers below still list them.
	seen := make(map[string]struct{}, len(under))
	out := make([]string, 0, len(under))
	for _, f := range under {
		if _, ok := seen[f]; !ok {
			seen[f] = struct{}{}
			out = append(out, f)
		}
	}
	prefix := repo + "/" + arch + "/"
	ptrs, err := p.kv.List(poolPtrNS)
	if err != nil {
		return nil, err
	}
	for _, e := range ptrs {
		if !strings.HasPrefix(e.Key, prefix) {
			continue
		}
		name := strings.TrimPrefix(e.Key, prefix)
		if _, ok := seen[name]; !ok {
			seen[name] = struct{}{}
			out = append(out, name)
		}
	}
	return out, nil
}

// RepoNames hides the reserved pool repo from the servable repo set.
func (p *poolStore) RepoNames() ([]string, error) {
	names, err := p.Store.RepoNames()
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(names))
	for _, n := range names {
		if n != poolRepo {
			out = append(out, n)
		}
	}
	return out, nil
}

// Arches hides the reserved pool repo (its lone "objects" dir is not a pacman arch).
func (p *poolStore) Arches(repo string) ([]string, error) {
	if repo == poolRepo {
		return []string{}, nil
	}
	return p.Store.Arches(repo)
}

// CollectPool removes pool objects no repo pointer references, subject to the
// retention policy. Safe to run online: it re-reads the reference set right before
// deleting, skips anything referenced again, and only deletes past the grace window,
// so an object briefly re-referenced by a concurrent upload is kept. A non-zero
// RetentionWindow closes the residual pointer-write race, since a just-added pointer
// means the object was referenced within the window and is skipped.
func (p *poolStore) CollectPool(ctx context.Context, policy PoolPolicy) (PoolGCResult, error) {
	var res PoolGCResult

	referenced, err := p.referencedHashes()
	if err != nil {
		return res, err
	}
	objs, err := p.kv.List(poolObjNS)
	if err != nil {
		return res, err
	}
	now := p.now()

	type candidate struct {
		hash      string
		group     string
		createdAt int64
	}
	var candidates []candidate

	for _, e := range objs {
		if err := ctx.Err(); err != nil {
			return res, err
		}
		res.Scanned++
		hash := e.Key
		var obj poolObject
		_ = json.Unmarshal(e.Value, &obj)

		if _, ok := referenced[hash]; ok {
			// Re-referenced: clear any stale unref marker so the grace clock restarts
			// if it drops again.
			if obj.UnrefAt != 0 {
				obj.UnrefAt = 0
				if b, mErr := json.Marshal(obj); mErr == nil {
					_ = p.kv.Set(poolObjNS, hash, b, 0)
				}
			}
			res.KeptReferenced++
			continue
		}
		// Unreferenced: start the grace clock on first observation, then measure the
		// window from it (so a zero window makes the object eligible immediately).
		unrefAt := obj.UnrefAt
		if unrefAt == 0 {
			unrefAt = now.UnixNano()
			obj.UnrefAt = unrefAt
			if b, mErr := json.Marshal(obj); mErr == nil {
				_ = p.kv.Set(poolObjNS, hash, b, 0)
			}
		}
		if now.Sub(time.Unix(0, unrefAt)) < policy.RetentionWindow {
			res.KeptRetained++
			continue
		}
		candidates = append(candidates, candidate{hash: hash, group: pkgGroup(obj.Filename), createdAt: obj.CreatedAt})
	}

	// Keep the newest KeepUnreferenced versions per group; the rest are deletable.
	byGroup := map[string][]candidate{}
	for _, c := range candidates {
		byGroup[c.group] = append(byGroup[c.group], c)
	}
	var toDelete []string
	for _, group := range byGroup {
		sort.Slice(group, func(i, j int) bool { return group[i].createdAt > group[j].createdAt })
		for i, c := range group {
			if i < policy.KeepUnreferenced {
				res.KeptRetained++
				continue
			}
			toDelete = append(toDelete, c.hash)
		}
	}
	if len(toDelete) == 0 {
		return res, nil
	}

	// Re-read references immediately before deleting so a hash re-pointed during the
	// scan is not collected.
	fresh, err := p.referencedHashes()
	if err != nil {
		return res, err
	}
	for _, hash := range toDelete {
		if err := ctx.Err(); err != nil {
			return res, err
		}
		if _, ok := fresh[hash]; ok {
			res.KeptReferenced++
			continue
		}
		if err := p.Store.DeleteFile(poolRepo, poolArch, hash); err != nil && !errors.Is(err, blob.ErrNotFound) {
			return res, errors.WrapErr(err, "pool gc: delete object")
		}
		if err := p.kv.Delete(poolObjNS, hash); err != nil {
			return res, errors.WrapErr(err, "pool gc: delete manifest")
		}
		res.Deleted++
	}
	return res, nil
}

// referencedHashes is the set of pool hashes at least one repo pointer points at.
func (p *poolStore) referencedHashes() (map[string]struct{}, error) {
	ptrs, err := p.kv.List(poolPtrNS)
	if err != nil {
		return nil, err
	}
	set := make(map[string]struct{}, len(ptrs))
	for _, e := range ptrs {
		set[string(e.Value)] = struct{}{}
	}
	return set, nil
}

// pkgGroup is the retention grouping key: a package's pkgname (versions of one
// package share it). An unrecognized name is its own group.
func pkgGroup(filename string) string {
	if m := pkgNameRe.FindStringSubmatch(filename); m != nil {
		return m[1]
	}
	return filename
}

// hashSeek streams file through SHA-256 and returns the hex digest and byte size;
// rewinding the file afterward is the caller's job (ensureObject re-seeks).
func hashSeek(file stream.SeekFile) (string, int64, error) {
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return "", 0, errors.WrapErr(err, "pool: seek package")
	}
	h := sha256.New()
	n, err := io.Copy(h, file)
	if err != nil {
		return "", 0, errors.WrapErr(err, "pool: hash package")
	}
	return hex.EncodeToString(h.Sum(nil)), n, nil
}
