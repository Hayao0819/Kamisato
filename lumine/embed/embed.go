package embed

import (
	"embed"
	"io/fs"
)

//go:embed out/**
var nextFS embed.FS

func NextFS() fs.FS {
	return nextFS
}
