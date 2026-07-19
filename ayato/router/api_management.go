package router

import (
	"github.com/gin-gonic/gin"

	"github.com/Hayao0819/Kamisato/ayato/handler"
	"github.com/Hayao0819/Kamisato/ayato/middleware"
)

func setPublicationRoutes(
	api *gin.RouterGroup,
	publications *handler.PublicationHandler,
	middlewares *middleware.Middleware,
) {
	upload := api.Group("")
	upload.Use(
		standardAPILimit(middlewares),
		middlewares.RequireCI(),
	)
	upload.POST("/repos/:repo/packages", publications.BatchUploadHandler)
	upload.POST("/repos/:repo/packages/presign", publications.PresignUploadHandler)
	upload.POST("/repos/:repo/packages/finalize", publications.FinalizeUploadHandler)

	remove := api.Group("")
	remove.Use(
		standardAPILimit(middlewares),
		middlewares.RequireCI(),
	)
	remove.DELETE("/repos/:repo/:arch/packages/:name", publications.BlinkyRemoveHandler)
	remove.DELETE("/repos/:repo/packages/:name", publications.BlinkyRemoveHandler)

	management := api.Group("")
	management.Use(
		standardAPILimit(middlewares),
		middlewares.RequireAdmin(),
	)
	management.POST("/repos/:repo/promote", publications.PromoteHandler)
	management.POST("/repos/:repo/sync-upstream", publications.SyncUpstreamHandler)
}

func setAdministrationRoutes(
	api *gin.RouterGroup,
	handlers *handler.Set,
	middlewares *middleware.Middleware,
) {
	admins := api.Group("/auth/admins")
	admins.Use(
		standardAPILimit(middlewares),
		middlewares.RequireAdmin(),
	)
	admins.GET("", handlers.Admins.AdminsListHandler)
	admins.POST("", handlers.Admins.AdminsAddHandler)
	admins.DELETE("/:id", handlers.Admins.AdminsRemoveHandler)

	signers := api.Group("/auth/signers")
	signers.Use(standardAPILimit(middlewares))
	signers.GET("", middlewares.RequireAdmin(), handlers.Signers.ListSignersHandler)
	signers.POST("", middlewares.RequireSignerRegistration(), handlers.Signers.RegisterSignerHandler)
	signers.DELETE("/:fingerprint", middlewares.RequireAdmin(), handlers.Signers.UnregisterSignerHandler)
}

func setBlinkyRoutes(
	engine *gin.Engine,
	publications *handler.PublicationHandler,
	middlewares *middleware.Middleware,
) {
	blinky := engine.Group("/blinky/api/unstable")
	blinky.Use(
		standardAPILimit(middlewares),
		middlewares.RequireBlinkyAdmin(),
	)
	blinky.PUT("/:repo/package", publications.BlinkyUploadHandler)
	blinky.DELETE("/:repo/package/:name", publications.BlinkyRemoveHandler)
}

func setRepositoryRoutes(engine *gin.Engine, repositories *handler.RepositoryHandler) error {
	repo := engine.Group("/repo")
	if err := setRepoAssets(repo); err != nil {
		return err
	}
	repo.GET("/:repo/mirrorlist", repositories.MirrorlistHandler)
	repo.GET("/:repo/:arch", repositories.RepoFileListHandler)
	repo.GET("/:repo/:arch/:file", repositories.RepoFileHandler)
	return nil
}
