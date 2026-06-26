package aurweb

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

// DumpSource is an optional Upstream capability: the bulk package dumps an AUR
// helper downloads in its default (non-RPC) discovery mode. When the upstream
// implements it, the Server serves dumps that merge local packages over the
// upstream set, so a helper pointed at this host still sees the full AUR.
type DumpSource interface {
	// DumpReader streams the decompressed JSON array of the upstream meta dump.
	// ext selects the -ext- variant that carries dependency arrays.
	DumpReader(ctx context.Context, ext bool) (io.ReadCloser, error)
	// FetchNames returns the upstream package-name list.
	FetchNames(ctx context.Context) ([]string, error)
}

const dumpTTL = 30 * time.Minute

// maxDumpBytes caps the decompressed upstream dump we will read, bounding memory
// against a decompression bomb from a hostile or compromised upstream.
const maxDumpBytes = 256 << 20

type dumpEntry struct {
	body    []byte
	etag    string
	expires time.Time
}

type dumpCache struct {
	mu      sync.Mutex
	entries map[string]dumpEntry

	// build serializes cache-miss builds so a cold cache doesn't trigger one
	// full upstream fetch per concurrent request (single-flight).
	build sync.Mutex
}

func (c *dumpCache) get(key string, now time.Time) (dumpEntry, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.entries[key]
	if !ok || now.After(e.expires) {
		return dumpEntry{}, false
	}
	return e, true
}

func (c *dumpCache) put(key string, e dumpEntry) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.entries == nil {
		c.entries = map[string]dumpEntry{}
	}
	c.entries[key] = e
}

// getOrBuild returns a fresh cached entry, building it under a single-flight
// lock (with a double-check) on a miss.
func (c *dumpCache) getOrBuild(key string, now time.Time, build func() ([]byte, error)) (dumpEntry, error) {
	if e, ok := c.get(key, now); ok {
		return e, nil
	}
	c.build.Lock()
	defer c.build.Unlock()
	if e, ok := c.get(key, now); ok {
		return e, nil
	}
	body, err := build()
	if err != nil {
		return dumpEntry{}, err
	}
	sum := md5.Sum(body)
	e := dumpEntry{body: body, etag: `"` + hex.EncodeToString(sum[:]) + `"`, expires: now.Add(dumpTTL)}
	c.put(key, e)
	return e, nil
}

func (s *Server) serveMetaDump(w http.ResponseWriter, r *http.Request, ext bool) {
	key := "meta"
	if ext {
		key = "meta-ext"
	}
	s.serveDump(w, r, key, func() ([]byte, error) { return s.buildMetaDump(r.Context(), ext) })
}

func (s *Server) serveNamesDump(w http.ResponseWriter, r *http.Request) {
	s.serveDump(w, r, "names", func() ([]byte, error) { return s.buildNamesDump(r.Context()) })
}

func (s *Server) serveDump(w http.ResponseWriter, r *http.Request, key string, build func() ([]byte, error)) {
	entry, err := s.dumps.getOrBuild(key, time.Now(), build)
	if err != nil {
		s.log.Error("aurweb: build dump", "key", key, "error", err)
		http.Error(w, "dump unavailable", http.StatusBadGateway)
		return
	}

	w.Header().Set("ETag", entry.etag)
	if r.Header.Get("If-None-Match") == entry.etag {
		w.WriteHeader(http.StatusNotModified)
		return
	}
	w.Header().Set("Content-Type", "application/gzip")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(entry.body)
}

// buildMetaDump writes a gzipped JSON array of every local package followed by
// the upstream packages a local package does not shadow. Upstream elements pass
// through unchanged; only their Name is read, so the full set is never resident.
func (s *Server) buildMetaDump(ctx context.Context, ext bool) ([]byte, error) {
	local, err := s.backend.All(ctx)
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	if _, err := gz.Write([]byte("[")); err != nil {
		return nil, err
	}

	first := true
	localNames := make(map[string]bool, len(local))
	for _, p := range local {
		localNames[p.Name] = true
		elem, mErr := json.Marshal(p.toMap(ext))
		if mErr != nil {
			return nil, mErr
		}
		if err := writeElem(gz, &first, elem); err != nil {
			return nil, err
		}
	}

	if ds, ok := s.upstream.(DumpSource); ok {
		rc, derr := ds.DumpReader(ctx, ext)
		if derr != nil {
			s.log.Warn("aurweb: upstream dump unavailable", "error", derr)
		} else {
			defer func() { _ = rc.Close() }()
			if err := streamUpstream(gz, io.LimitReader(rc, maxDumpBytes), localNames, &first); err != nil {
				s.log.Warn("aurweb: upstream dump stream truncated", "error", err)
			}
		}
	}

	if _, err := gz.Write([]byte("]")); err != nil {
		return nil, err
	}
	if err := gz.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// streamUpstream copies upstream array elements into gz, skipping any whose Name
// a local package already provided.
func streamUpstream(gz io.Writer, rc io.Reader, skip map[string]bool, first *bool) error {
	dec := json.NewDecoder(rc)
	if _, err := dec.Token(); err != nil { // opening '['
		return err
	}
	for dec.More() {
		var raw json.RawMessage
		if err := dec.Decode(&raw); err != nil {
			return err
		}
		var nm struct{ Name string }
		if json.Unmarshal(raw, &nm) == nil && skip[nm.Name] {
			continue
		}
		if err := writeElem(gz, first, raw); err != nil {
			return err
		}
	}
	return nil
}

func writeElem(w io.Writer, first *bool, elem []byte) error {
	if !*first {
		if _, err := w.Write([]byte(",")); err != nil {
			return err
		}
	}
	*first = false
	_, err := w.Write(elem)
	return err
}

func (s *Server) buildNamesDump(ctx context.Context) ([]byte, error) {
	local, err := s.backend.All(ctx)
	if err != nil {
		return nil, err
	}

	seen := make(map[string]bool, len(local))
	var names []string
	for _, p := range local {
		if !seen[p.Name] {
			seen[p.Name] = true
			names = append(names, p.Name)
		}
	}
	if ds, ok := s.upstream.(DumpSource); ok {
		up, derr := ds.FetchNames(ctx)
		if derr != nil {
			s.log.Warn("aurweb: upstream names unavailable", "error", derr)
		} else {
			for _, n := range up {
				if !seen[n] {
					seen[n] = true
					names = append(names, n)
				}
			}
		}
	}

	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	if _, err := gz.Write([]byte(strings.Join(names, "\n") + "\n")); err != nil {
		return nil, err
	}
	if err := gz.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
