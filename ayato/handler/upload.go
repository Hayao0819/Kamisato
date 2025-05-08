package handler

import (
	"fmt"
	"log"
	"net/http"

	"github.com/Hayao0819/Kamisato/ayato/utils"
	"github.com/gin-gonic/gin"
)

func UploadHandler(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		c.String(http.StatusBadRequest, fmt.Sprintf("get form err: %s", err.Error()))
		return
	}

	filename := file.Filename
	log.Println(filename)

	// Save the file to the repository
	filePath := config.RepoPath + "/" + filename
	err = c.SaveUploadedFile(file, filePath)
	if err != nil {
		c.String(http.StatusInternalServerError, fmt.Sprintf("upload file err: %s", err.Error()))
		return
	}

	// Add the package to the repository database
	useSignedDB := false
	var gnupgDir *string
	err = utils.RepoAdd(config.DBPath, filePath, useSignedDB, gnupgDir)
	if err != nil {
		c.String(http.StatusInternalServerError, fmt.Sprintf("repo-add err: %s", err.Error()))
		return
	}

	c.String(http.StatusOK, fmt.Sprintf("'%s' uploaded!", filename))
}
