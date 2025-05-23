package embed

import (
	"embed"
	"fmt"
	"io/fs"
	"net/http"
)

//go:embed out/**
var nextFS embed.FS

func NextFS() fs.FS {
	return nextFS
}

func NextHandler() (http.Handler, error) {
	staticFS, err := fs.Sub(NextFS(), "out")
	if err != nil {
		return nil, fmt.Errorf("failed to prepare embedded filesystem: %w", err)
	}

	fileServer := http.FileServer(http.FS(staticFS))
	return fileServer, nil
}
