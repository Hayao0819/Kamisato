package handler

import (
	"fmt"
	"log/slog"
	"net/http"

	"github.com/Hayao0819/Kamisato/ayato/domain"
	"github.com/Hayao0819/Kamisato/ayato/stream"
	"github.com/gin-gonic/gin"
)

// BuildPackageHandler handles package build requests
func (h *Handler) BuildPackageHandler(ctx *gin.Context) {
	repoName := ctx.Param("repo")
	if repoName == "" {
		ctx.JSON(http.StatusBadRequest, domain.APIError{Message: "repository name is required"})
		return
	}

	// Parse multipart form
	if err := ctx.Request.ParseMultipartForm(100 << 20); err != nil { // 100MB max
		ctx.JSON(http.StatusBadRequest, domain.APIError{Message: fmt.Sprintf("parse form err: %s", err.Error())})
		return
	}

	// Get PKGBUILD file
	pkgbuildHeader, err := formFileWithValidate(ctx, "pkgbuild", 1<<20) // 1MB max for PKGBUILD
	if err != nil {
		ctx.JSON(http.StatusBadRequest, domain.APIError{Message: fmt.Sprintf("PKGBUILD file is required: %s", err.Error())})
		return
	}

	pkgbuildStream, err := formFileStream(pkgbuildHeader)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, domain.APIError{Message: fmt.Sprintf("failed to open PKGBUILD: %s", err.Error())})
		return
	}
	defer pkgbuildStream.Close()

	// Get optional parameters
	arch := ctx.PostForm("arch")
	if arch == "" {
		arch = "x86_64" // default
	}
	gpgKey := ctx.PostForm("gpgkey")

	// Get additional files if any
	form := ctx.Request.MultipartForm
	var additionalFiles []*stream.FileStream
	if form != nil && form.File != nil {
		for fieldName, headers := range form.File {
			// Skip the PKGBUILD field
			if fieldName == "pkgbuild" {
				continue
			}

			// Process each additional file
			for _, header := range headers {
				fileStream, err := formFileStream(header)
				if err != nil {
					slog.Warn("failed to open additional file", "file", header.Filename, "error", err)
					continue
				}
				additionalFiles = append(additionalFiles, fileStream)
			}
		}
	}

	// Ensure additional files are closed after use
	defer func() {
		for _, file := range additionalFiles {
			if file != nil {
				file.Close()
			}
		}
	}()

	// Create build request
	buildReq := &domain.BuildRequest{
		PKGBUILD:        pkgbuildStream,
		AdditionalFiles: additionalFiles,
		Arch:            arch,
		GPGKey:          gpgKey,
	}

	// Execute build
	slog.Info("starting package build", "repo", repoName, "arch", arch)
	if err := h.s.BuildPackage(repoName, buildReq); err != nil {
		slog.Error("build failed", "repo", repoName, "error", err)
		ctx.JSON(http.StatusInternalServerError, domain.APIError{Message: fmt.Sprintf("build failed: %s", err.Error())})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"message": "Package built and uploaded successfully",
		"repo":    repoName,
		"arch":    arch,
	})
}
