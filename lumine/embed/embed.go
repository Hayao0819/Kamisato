package embed

import (
	"embed"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
)

//go:embed all:out
var nextFS embed.FS

func NextFS() fs.FS {
	return nextFS
}

func NextHandler() (http.Handler, error) {
	staticFS, err := fs.Sub(NextFS(), "out")
	if err != nil {
		return nil, fmt.Errorf("failed to prepare embedded filesystem: %w", err)
	}

	// A plain `go build` embeds an empty out/ (only .gitkeep); warn loudly rather
	// than silently serving a blank SPA. The bundle is produced by the Dockerfile
	// web-builder stage / install.sh.
	if _, err := fs.Stat(staticFS, "index.html"); err != nil {
		slog.Warn("embedded web bundle is missing index.html; lumine will serve a blank page — build the frontend into lumine/embed/out")
	}

	fileServer := http.FileServer(http.FS(staticFS))
	return fileServer, nil
}
