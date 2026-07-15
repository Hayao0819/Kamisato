package service

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"

	"github.com/Hayao0819/Kamisato/ayato/domain"
	pacmanrepo "github.com/Hayao0819/Kamisato/pkg/pacman/repo"
)

// Repository databases are normally far smaller than this. Bounding the body
// prevents a broken or hostile configured mirror from exhausting ayato's heap.
const maxUpstreamDBBytes = 512 << 20

// Syncer refreshes an upstream-layered repo from its upstream database.
type Syncer interface {
	SyncUpstream(ctx context.Context, repo string) (UpstreamSyncResult, error)
}

// UpstreamSyncResult reports, per architecture, whether the upstream changed and
// how many packages were added/removed/updated relative to the last snapshot.
type UpstreamSyncResult struct {
	Repo   string           `json:"repo"`
	Arches []ArchSyncResult `json:"arches"`
}

// ArchSyncResult is one architecture's outcome. Changed is false when the
// upstream was unchanged (a conditional-GET 304), making that arch a cheap no-op.
type ArchSyncResult struct {
	Arch    string `json:"arch"`
	Changed bool   `json:"changed"`
	Added   int    `json:"added,omitempty"`
	Removed int    `json:"removed,omitempty"`
	Updated int    `json:"updated,omitempty"`
	Error   string `json:"error,omitempty"`
}

// SyncUpstream refreshes an upstream-layered repo: per arch it conditional-GETs the
// upstream .db (unchanged is a cheap no-op) and, on a change, records the snapshot
// and rebuilds the merged db with the local overlay re-applied. Best-effort: a
// per-arch failure is recorded and skipped, never breaking the last-good merged db.
func (s *Service) SyncUpstream(ctx context.Context, repo string) (UpstreamSyncResult, error) {
	rc := s.cfg.ResolveRepo(repo)
	if rc == nil || !rc.Upstream.Enabled() {
		return UpstreamSyncResult{}, fmt.Errorf("%w: %q has no upstream configured", domain.ErrInvalid, repo)
	}
	useSignedDB := s.signedDB()
	res := UpstreamSyncResult{Repo: rc.Name}
	for _, arch := range s.repoArches(rc.Name) {
		if err := ctx.Err(); err != nil {
			return res, err
		}
		ar := s.syncUpstreamArch(ctx, rc.Name, arch, rc.Upstream.DBURLFor(arch), rc.Upstream.FilesURLFor(arch), useSignedDB)
		res.Arches = append(res.Arches, ar)
	}
	return res, nil
}

func (s *Service) syncUpstreamArch(ctx context.Context, repo, arch, dbURL, filesURL string, useSignedDB bool) ArchSyncResult {
	ar := ArchSyncResult{Arch: arch}
	etag, lastMod, err := s.pkgBinaryRepo.UpstreamValidators(repo, arch)
	if err != nil {
		ar.Error = err.Error()
		return ar
	}
	dbGz, newETag, newLastMod, changed, err := s.conditionalGet(ctx, dbURL, etag, lastMod)
	if err != nil {
		slog.Warn("upstream sync: fetch db", "repo", repo, "arch", arch, "err", err)
		ar.Error = err.Error()
		return ar
	}
	if !changed {
		return ar // 304: unchanged, serving the last-good merged db
	}
	// The files db is optional: a merged files listing is nice-to-have, so a fetch
	// failure just leaves it absent rather than failing the sync.
	filesGz, _, _, filesChanged, ferr := s.conditionalGet(ctx, filesURL, "", "")
	if ferr != nil || !filesChanged {
		filesGz = nil
	}

	diff, err := s.pkgBinaryRepo.ApplyUpstreamSnapshot(repo, arch, dbGz, filesGz, newETag, newLastMod, useSignedDB)
	if err != nil {
		slog.Error("upstream sync: apply snapshot", "repo", repo, "arch", arch, "err", err)
		ar.Error = err.Error()
		return ar
	}
	ar.Changed = true
	ar.Added = len(diff.Added)
	ar.Removed = len(diff.Removed)
	ar.Updated = len(diff.Updated)
	slog.Info("upstream synced", "repo", repo, "arch", arch, "added", ar.Added, "removed", ar.Removed, "updated", ar.Updated)
	return ar
}

// conditionalGet fetches url, sending the stored validators so an unchanged
// upstream answers 304. It returns the body and fresh validators when the
// resource changed (changed=true), or (nil, "", "", false, nil) on a 304.
func (s *Service) conditionalGet(ctx context.Context, url, etag, lastMod string) (body []byte, newETag, newLastMod string, changed bool, err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, "", "", false, err
	}
	if etag != "" {
		req.Header.Set("If-None-Match", etag)
	}
	if lastMod != "" {
		req.Header.Set("If-Modified-Since", lastMod)
	}
	resp, err := s.upstreamClient.Do(req)
	if err != nil {
		return nil, "", "", false, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotModified {
		return nil, "", "", false, nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, "", "", false, fmt.Errorf("upstream returned %s for %s", resp.Status, url)
	}
	if resp.ContentLength > maxUpstreamDBBytes {
		return nil, "", "", false, fmt.Errorf("upstream response exceeds %d bytes for %s", maxUpstreamDBBytes, url)
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, maxUpstreamDBBytes+1))
	if err != nil {
		return nil, "", "", false, err
	}
	if len(b) > maxUpstreamDBBytes {
		return nil, "", "", false, fmt.Errorf("upstream response exceeds %d bytes for %s", maxUpstreamDBBytes, url)
	}
	return b, resp.Header.Get("ETag"), resp.Header.Get("Last-Modified"), true, nil
}

func (s *Service) isUpstreamRepo(repo string) bool {
	if s.cfg == nil {
		return false
	}
	rc := s.cfg.ResolveRepo(repo)
	return rc != nil && rc.Upstream.Enabled()
}

// overlayRepo reads the LOCAL overlay database, bypassing the merged view. The
// upload version gate compares against what an upload would replace — the local
// overlay — so an upstream package can still be shadowed at the same or a lower
// version. A plain repo's served db IS its overlay, so it just reads RemoteRepo.
func (s *Service) overlayRepo(repo, arch string) (*pacmanrepo.RemoteRepo, error) {
	if !s.isUpstreamRepo(repo) {
		return s.pkgBinaryRepo.RemoteRepo(repo, arch)
	}
	f, err := s.pkgBinaryRepo.FetchFile(repo, arch, repo+".db.tar.gz")
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return pacmanrepo.RemoteRepoFromDB(repo, f)
}

// rebuildMergedIfUpstream refreshes the served merged database after a local
// publish or removal, so a locally published package shows up in an upstream
// repo's served view without waiting for the next upstream sync. Best-effort: a
// rebuild failure is logged and leaves the last-good merged db in place.
func (s *Service) rebuildMergedIfUpstream(repo string, arches []string) {
	if !s.isUpstreamRepo(repo) {
		return
	}
	useSignedDB := s.signedDB()
	for _, arch := range arches {
		if err := s.pkgBinaryRepo.RebuildMerged(repo, arch, useSignedDB); err != nil {
			slog.Warn("rebuild merged db after local change", "repo", repo, "arch", arch, "err", err)
		}
	}
}
