package handler

import (
	"net/http"
	"os"
	"path/filepath"

	"github.com/gin-gonic/gin"
)

// ListArtifactsHandler lists the downloadable files of a client-signed job.
func (h *Handler) ListArtifactsHandler(c *gin.Context) {
	dir, err := h.s.ArtifactDir(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		// Skip dirs and symlinks: only regular build outputs are downloadable.
		if !e.IsDir() && e.Type()&os.ModeSymlink == 0 {
			names = append(names, e.Name())
		}
	}
	c.JSON(http.StatusOK, gin.H{"artifacts": names})
}

// GetArtifactHandler serves one artifact file by name.
func (h *Handler) GetArtifactHandler(c *gin.Context) {
	dir, err := h.s.ArtifactDir(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	name := c.Param("name")
	// Only a single safe path component may be served.
	if name == "" || name != filepath.Base(name) || name == "." || name == ".." {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid artifact name"})
		return
	}
	full := filepath.Join(dir, name)
	// Lstat (not Stat) so a symlink an untrusted build planted is rejected rather
	// than followed out of the artifact dir.
	if info, statErr := os.Lstat(full); statErr != nil || info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "artifact not found"})
		return
	}
	c.File(full)
}
