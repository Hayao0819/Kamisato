package aurweb

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"net/http"
)

func (s *Server) writeResults(w http.ResponseWriter, r *http.Request, callback, typ string, pkgs []Pkg, info bool) {
	results := make([]map[string]any, len(pkgs))
	for i, p := range pkgs {
		results[i] = p.toMap(info)
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

// versionOrNull renders the RPC version field: the client's value, or JSON null
// when it was omitted (version 0), the way aurweb echoes it.
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

	sum := md5.Sum(body)
	etag := `"` + hex.EncodeToString(sum[:]) + `"`
	w.Header().Set("ETag", etag)
	if match := r.Header.Get("If-None-Match"); match == etag {
		w.WriteHeader(http.StatusNotModified)
		return
	}

	// aurweb answers with HTTP 200 even for app-level errors. Set it explicitly:
	// a host may have preset another status (gin's NoRoute defaults to 404).
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
