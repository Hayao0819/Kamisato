package aurweb

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/md5" //nolint:gosec // md5 is a non-crypto ETag/cache hash, not a security primitive
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

// DumpSource is an optional Upstream capability for bulk package dumps AUR helpers use in discovery mode.
// When implemented, the Server merges local packages over the upstream so helpers pointed here see the full AUR.
type DumpSource interface {
	// DumpReader streams the decompressed JSON array of the upstream meta dump.
	// ext selects the -ext- variant that carries dependency arrays.
	DumpReader(ctx context.Context, ext bool) (io.ReadCloser, error)
	// FetchNames returns the upstream package-name list.
	FetchNames(ctx context.Context) ([]string, error)
}

const dumpTTL = 30 * time.Minute

// maxDumpBytes caps decompressed upstream dump size to bound memory against a decompression bomb.
const maxDumpBytes = 256 << 20

type dumpEntry struct {
	body    []byte
	etag    string
	expires time.Time
}

type dumpCache struct {
	mu      sync.Mutex
	entries map[string]dumpEntry

	// build serializes cache-miss builds (single-flight) so a cold cache triggers only one upstream fetch.
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

// getOrBuild returns a fresh cached entry, using a single-flight lock with double-check on miss.
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
	sum := md5.Sum(body) //nolint:gosec // ETag over the response body, not a security hash
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

// buildMetaDump writes a gzipped JSON array of all local packages plus upstream packages not shadowed locally;
// only %NAME% is read from upstream elements so the full set is never in memory.
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
		elem, mErr := json.Marshal(p.result(ext))
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
			// One byte of slack: lr.N > 0 = complete, lr.N = 0 = truncated by cap.
			// A truncated upstream must not be cached as complete or clients get an incomplete set for the TTL.
			lr := &io.LimitedReader{R: rc, N: maxDumpBytes + 1}
			if err := streamUpstream(gz, lr, localNames, &first); err != nil {
				return nil, fmt.Errorf("aurweb: upstream dump stream failed, refusing to cache: %w", err)
			}
			if lr.N <= 0 {
				return nil, fmt.Errorf("aurweb: upstream dump exceeds the %d-byte cap, refusing to cache a truncated set", maxDumpBytes)
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

// streamUpstream copies upstream JSON array elements into gz, skipping names already provided locally.
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

	names := make([]string, 0, len(local))
	for _, p := range local {
		names = append(names, p.Name)
	}
	if ds, ok := s.upstream.(DumpSource); ok {
		up, derr := ds.FetchNames(ctx)
		if derr != nil {
			s.log.Warn("aurweb: upstream names unavailable", "error", derr)
		} else {
			names = append(names, up...)
		}
	}
	// Local names lead, so DedupeBy keeps a local name over an upstream duplicate.
	names = DedupeBy(names, func(n string) string { return n })

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
