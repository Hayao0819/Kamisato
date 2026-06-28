package aurweb

import (
	"encoding/json"
	"net/http"
	"strings"
)

// rpcQuery is a parsed RPC request from any of the supported request shapes.
type rpcQuery struct {
	version  int
	typ      string
	by       By
	args     []string
	callback string
}

func parseRPC(r *http.Request) rpcQuery {
	_ = r.ParseForm()
	var q rpcQuery

	path := r.URL.Path
	var pathArg string
	if strings.HasPrefix(path, "/rpc/v") {
		segs := strings.Split(strings.Trim(strings.TrimPrefix(path, "/rpc/"), "/"), "/")
		if len(segs) > 0 {
			q.version = parseVersion(segs[0])
		}
		if len(segs) > 1 {
			q.typ = segs[1]
		}
		if len(segs) > 2 {
			pathArg = strings.Join(segs[2:], "/")
		}
		if r.Method == http.MethodPost && strings.Contains(r.Header.Get("Content-Type"), "json") {
			if data, _ := readAllLimited(r.Body); len(data) > 0 {
				var body struct {
					By  string          `json:"by"`
					Arg json.RawMessage `json:"arg"`
				}
				if json.Unmarshal(data, &body) == nil {
					if body.By != "" {
						q.by = By(body.By)
					}
					q.args = append(q.args, parseJSONArg(body.Arg)...)
				}
			}
		}
	} else {
		q.version = atoiSafe(r.Form.Get("v"))
		q.typ = r.Form.Get("type")
	}

	q.callback = r.Form.Get("callback")
	if by := r.Form.Get("by"); by != "" {
		q.by = By(by)
	}
	if vals, ok := r.Form["arg[]"]; ok {
		q.args = append(q.args, vals...)
	}
	if a := r.Form.Get("arg"); a != "" {
		q.args = append(q.args, a)
	}
	// aurweb ignores the OpenAPI path arg when query/body args are present.
	if len(q.args) == 0 && pathArg != "" {
		q.args = append(q.args, pathArg)
	}
	return q
}

// parseJSONArg accepts both the string and []string spellings of the OpenAPI
// POST "arg" field.
func parseJSONArg(raw json.RawMessage) []string {
	if len(raw) == 0 {
		return nil
	}
	var list []string
	if json.Unmarshal(raw, &list) == nil {
		return list
	}
	var single string
	if json.Unmarshal(raw, &single) == nil && single != "" {
		return []string{single}
	}
	return nil
}

// resolveType maps aurweb's type aliases to a canonical handler key.
func resolveType(t string) string {
	switch t {
	case "info", "multiinfo":
		return "multiinfo"
	case "search":
		return "search"
	case "msearch":
		return "msearch"
	case "suggest":
		return "suggest"
	case "suggest-pkgbase":
		return "suggest-pkgbase"
	default:
		return ""
	}
}
