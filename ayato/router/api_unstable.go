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
func SetRoute(e *gin.Engine, h *handler.Handler, m *middleware.Middleware) error {
	// テンプレート設定
	if err := view.Set(e); err != nil {
		return utils.WrapErr(err, "テンプレート設定に失敗")
	}

	// miko への逆プロキシ。ビルド/ジョブは miko が単一の状態源として保持し、
	// ayato は API キーを付けて素通しするだけ（クライアントは miko に直接到達しない）。
	mikoProxy, err := h.MikoProxy()
	if err != nil {
		return utils.WrapErr(err, "miko プロキシの初期化に失敗")
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

		if mikoProxy != nil {
			// ジョブ状態の参照系は公開。静的な /jobs は既存の /:repo パラメータルートと
			// 共存できる（ayato は /repos と /:repo を既に併用している）。
			api.GET("/jobs", mikoProxy.Handler("/api/unstable/jobs"))
			api.GET("/jobs/:id", mikoProxy.HandlerFunc(func(c *gin.Context) string {
				return "/api/unstable/jobs/" + c.Param("id")
			}))
			api.GET("/jobs/:id/logs", mikoProxy.HandlerFunc(func(c *gin.Context) string {
				return "/api/unstable/jobs/" + c.Param("id") + "/logs"
			}))
		}

		auth := api.Group("")
		{
			auth.Use(m.BasicAuth)
			auth.PUT("/:repo/package", h.BlinkyUploadHandler)          // Blinky compatible
			auth.DELETE("/:repo/package/:name", h.BlinkyRemoveHandler) // Blinky compatible
			if mikoProxy != nil {
				// ビルド投入はクライアント認証の背後でのみ受け付ける。
				auth.POST("/build", mikoProxy.Handler("/api/unstable/build"))
			}
		}
	}
	{
		repo := e.Group("/repo")
		repo.GET("/:repo/:arch", h.RepoFileListHandler)
		repo.GET("/:repo/:arch/:file", h.RepoFileHandler)
	}
	return nil
}
