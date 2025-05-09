package handler

import (
	"fmt"
	"net/http"
	"path"

	"github.com/Hayao0819/Kamisato/ayato/middleware"
	"github.com/Hayao0819/Kamisato/ayato/utils"
	"github.com/Hayao0819/Kamisato/repo"
	"github.com/gin-gonic/gin"
)

func UploadHandler(ctx *gin.Context) {
	// Check if the request contains a file
	file, err := ctx.FormFile("file")
	if err != nil {
		ctx.String(http.StatusBadRequest, fmt.Sprintf("get form err: %s", err.Error()))
		return
	}

	// Get repository information
	repoName := ctx.Param("repo")
	// pkgName := ctx.Param("name")

	// Validate the file
	if err := repo.ValidatePackageBinary(file); err != nil {
		ctx.String(http.StatusBadRequest, fmt.Sprintf("validate file err: %s", err.Error()))
		return
	}

	// Get the Ayato configuration
	cfg := middleware.GetConfig(ctx)
	if cfg == nil {
		ctx.String(http.StatusInternalServerError, "Configuration not found")
		return
	}

	// Assemble the file path
	// TODO: 複数アーキテクチャに対応する
	var repoDbPath string // Path to the repository database
	for _, r := range cfg.RepoPath {
		if path.Base(r) == repoName {
			repoDbPath = r
			break
		}
	}
	filename := file.Filename
	fullPkgBinary := path.Join(repoDbPath, repoName, filename)

	// Save the file to the repository
	err = ctx.SaveUploadedFile(file, fullPkgBinary)
	if err != nil {
		ctx.String(http.StatusInternalServerError, fmt.Sprintf("upload file err: %s", err.Error()))
		return
	}

	// Add the package to the repository database
	useSignedDB := false
	var gnupgDir *string
	err = utils.RepoAdd(repoDbPath, fullPkgBinary, useSignedDB, gnupgDir)
	if err != nil {
		ctx.String(http.StatusInternalServerError, fmt.Sprintf("repo-add err: %s", err.Error()))
		return
	}

	ctx.String(http.StatusOK, fmt.Sprintf("'%s' uploaded!", filename))
}
