package aurweb

import (
	"crypto/md5" //nolint:gosec // md5 is a non-crypto ETag/cache hash, not a security primitive
	"encoding/hex"
	"encoding/json"
	"net/http"
)

func (s *Server) writeResults(w http.ResponseWriter, r *http.Request, callback, typ string, pkgs []Pkg, info bool) {
	results := make([]any, len(pkgs))
	for i, p := range pkgs {
		results[i] = p.result(info)
	}
	s.writeJSON(w, r, callback, map[string]any{
		"version":     Version,
		"type":        typ,
		"resultcount": len(results),
		"results":     results,
	})
}

func (s *Server) writeError(w http.ResponseWriter, r *http.Request, callback string, version int, msg string) {
	s.writeJSON(w, r, callback, map[string]any{
		"version":     versionOrNull(version),
		"type":        "error",
		"resultcount": 0,
		"results":     []any{},
		"error":       msg,
	})
}

// versionOrNull returns the client's RPC version or JSON null when omitted (version 0), mirroring aurweb.
func versionOrNull(version int) any {
	if version == 0 {
		return nil
	}
	return version
}

func (s *Server) writeJSON(w http.ResponseWriter, r *http.Request, callback string, payload any) {
	body, err := json.Marshal(payload)
	if err != nil {
		s.log.Error("aurweb: marshal response", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	sum := md5.Sum(body) //nolint:gosec // ETag over the response body, not a security hash
	etag := `"` + hex.EncodeToString(sum[:]) + `"`
	w.Header().Set("ETag", etag)
	if match := r.Header.Get("If-None-Match"); match == etag {
		w.WriteHeader(http.StatusNotModified)
		return
	}

	// aurweb returns HTTP 200 even for app-level errors; set it explicitly in case a host preset another status.
	if callback != "" {
		w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("/**/" + callback + "("))
		_, _ = w.Write(body)
		_, _ = w.Write([]byte(")"))
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(body)
}
