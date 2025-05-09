package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func RemoveHandler(ctx *gin.Context) {

	ctx.AbortWithStatus(http.StatusNotImplemented)

	// cfg := middleware.GetConfig(ctx)
	// if cfg == nil {
	// 	ctx.String(http.StatusInternalServerError, "Configuration not found")
	// 	return
	// }

	// packageName := ctx.Query("package")
	// if packageName == "" {
	// 	ctx.String(http.StatusBadRequest, "Package name is required")
	// 	return
	// }

	// // Remove the package from the repository
	// useSignedDB := false
	// var gnupgDir *string
	// err := utils.RepoRemove(cfg.DBPath, packageName, useSignedDB, gnupgDir)
	// if err != nil {
	// 	ctx.String(http.StatusInternalServerError, fmt.Sprintf("repo-remove err: %s", err.Error()))
	// 	return
	// }

	// ctx.String(http.StatusOK, fmt.Sprintf("'%s' removed!", packageName))
}
