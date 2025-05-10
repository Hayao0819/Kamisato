package handler

import (
	"fmt"
	"net/http"
	"path"

	"github.com/Hayao0819/Kamisato/ayato/utils"
	"github.com/Hayao0819/Kamisato/repo"
	"github.com/gin-gonic/gin"
)

func (h *Handler) UploadHandler(ctx *gin.Context) {
	// Check if the request contains a file
	repoName := ctx.Param("repo")
	file, err := ctx.FormFile("package")
	if err != nil {
		ctx.String(http.StatusBadRequest, fmt.Sprintf("get form err: %s", err.Error()))
		return
	}

	// TODO: Check signature file

	// Validate the file
	if err := repo.ValidatePkgHeader(file); err != nil {
		ctx.String(http.StatusBadRequest, fmt.Sprintf("validate file err: %s", err.Error()))
		return
	}

	// Assemble the file path
	// TODO: 複数アーキテクチャに対応する
	repoDbPath, err := determineRepoDir(h.cfg.RepoPath, repoName)
	if err != nil {
		ctx.String(http.StatusBadRequest, fmt.Sprintf("repo %s not found", repoName))
		return
	}
	fullPkgBinary := path.Join(repoDbPath, file.Filename)

	// Save the file to the repository
	if err := ctx.SaveUploadedFile(file, fullPkgBinary); err != nil {
		ctx.String(http.StatusInternalServerError, fmt.Sprintf("upload file err: %s", err.Error()))
		return
	}

	// Add the package to the repository database
	useSignedDB := false
	var gnupgDir *string // TODO: Check if the directory exists
	err = utils.RepoAdd(repoDbPath, fullPkgBinary, useSignedDB, gnupgDir)
	if err != nil {
		ctx.String(http.StatusInternalServerError, fmt.Sprintf("repo-add err: %s", err.Error()))
		return
	}

	ctx.String(http.StatusOK, fmt.Sprintf("'%s' uploaded!", file.Filename))
}

func determineRepoDir(repo []string, name string) (string, error) {
	for _, r := range repo {
		if path.Base(r) == name {
			return r, nil
		}
	}
	return "", fmt.Errorf("repo %s not found", name)
}
