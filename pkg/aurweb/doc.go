package aurweb

import "net/http"

// rpcDoc is the page a bare GET /rpc serves, mirroring aurweb's documentation().
const rpcDoc = `<!DOCTYPE html>
<html lang="en">
<head><meta charset="utf-8"><title>RPC Interface</title></head>
<body>
<h1>RPC Interface</h1>
<p>aurweb-compatible RPC. Queries are HTTP GET requests; responses are JSON.</p>
<h2>Endpoints</h2>
<ul>
<li><code>/rpc/?v=5&amp;type=search&amp;by=name&amp;arg=foo</code> — search (by: name, name-desc, maintainer, depends, makedepends, ...)</li>
<li><code>/rpc/?v=5&amp;type=info&amp;arg[]=foo&amp;arg[]=bar</code> — info for one or more packages</li>
<li><code>/rpc/?v=5&amp;type=suggest&amp;arg=fo</code> — name completion</li>
<li><code>/rpc/v5/search/foo?by=name</code> — same, OpenAPI-style path</li>
<li><code>/rpc/v5/info?arg[]=foo</code></li>
</ul>
<p>Only version 5 is supported. Results are capped, and the endpoint may be rate limited (HTTP 429).</p>
</body>
</html>
`

func (s *Server) serveRPCDoc(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(rpcDoc))
}
