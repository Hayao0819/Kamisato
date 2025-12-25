package router

import (
	"github.com/Hayao0819/Kamisato/ayato/handler"
	"github.com/Hayao0819/Kamisato/ayato/middleware"
	"github.com/Hayao0819/Kamisato/ayato/view"
	"github.com/Hayao0819/Kamisato/internal/utils"

	"github.com/gin-gonic/gin"
)

// SetRoute sets all API routes for the Ayato server.
// Ayatoサーバーの全APIルートを設定します。
func SetRoute(e *gin.Engine, h handler.IHandler, m *middleware.Middleware) error {
	// テンプレート設定
	if err := view.Set(e); err != nil {
		return utils.WrapErr(err, "テンプレート設定に失敗")
	}

	{
		api := e.Group("/api/unstable")
		api.Use(m.Cors())
		{
			api.GET("/hello", h.HelloHandler)
			api.GET("/teapot", h.TeapotHandler)
			api.GET("/auth/required", h.AuthRequiredHandler)
			api.GET("/repos", h.ReposHandler)
			api.GET("/repos/:repo/archs", h.ArchesHandler)
			api.GET("/:repo/:arch/package", h.AllPkgsHandler)
			api.GET("/:repo/:arch/package/:name", h.PkgDetailHandler) // TODO: 実装未完（詳細取得）
			api.GET("/:repo/:arch/package/:name/files", h.PkgFilesHandler)
		}

		auth := api.Group("")
		{
			auth.Use(m.BasicAuth)
			auth.PUT("/:repo/package", h.BlinkyUploadHandler)          // Blinky compatible
			auth.DELETE("/:repo/package/:name", h.BlinkyRemoveHandler) // Blinky compatible
		}
	}
	{
		repo := e.Group("/repo")
		repo.GET("/:repo/:arch", h.RepoFileListHandler)
		repo.GET("/:repo/:arch/:file", h.RepoFileHandler)
	}
	return nil
}
