package handler

import (
	"fmt"
	"net/http"

	"github.com/Hayao0819/Kamisato/ayato/utils"
	"github.com/gin-gonic/gin"
)

func RemoveHandler(c *gin.Context) {
	packageName := c.Query("package")
	if packageName == "" {
		c.String(http.StatusBadRequest, "Package name is required")
		return
	}

	// Remove the package from the repository
	useSignedDB := false
	var gnupgDir *string
	err := utils.RepoRemove(config.DBPath, packageName, useSignedDB, gnupgDir)
	if err != nil {
		c.String(http.StatusInternalServerError, fmt.Sprintf("repo-remove err: %s", err.Error()))
		return
	}

	c.String(http.StatusOK, fmt.Sprintf("'%s' removed!", packageName))
}
