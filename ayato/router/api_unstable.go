package router

import (
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"

	"github.com/Hayao0819/Kamisato/ayato/handler"
	"github.com/Hayao0819/Kamisato/ayato/middleware"
	"github.com/Hayao0819/Kamisato/ayato/router/view"
	"github.com/Hayao0819/Kamisato/internal/errors"
)

func SetRoute(
	engine *gin.Engine,
	handlers *handler.Set,
	middlewares *middleware.Middleware,
	options ...RouteOption,
) error {
	config := routeConfig{}
	for _, option := range options {
		option(&config)
	}
	if err := view.Set(engine); err != nil {
		return errors.WrapErr(err, "failed to configure templates")
	}

	setHealthRoutes(engine, config.readiness)
	mikoProxy, err := handlers.Miko.Proxy()
	if err != nil {
		return errors.WrapErr(err, "failed to initialize the miko proxy")
	}

	api := engine.Group("/api/unstable")
	api.Use(middlewares.Cors())
	authLimit := middlewares.RateLimit(rate.Every(time.Second/5), 20)

	setPublicRoutes(api, handlers, middlewares)
	setAuthRoutes(api, handlers.Auth, authLimit)
	setMikoRoutes(api, handlers.Miko, middlewares, mikoProxy)
	setPublicationRoutes(api, handlers.Publications, middlewares)
	setAdministrationRoutes(api, handlers, middlewares)
	setBlinkyRoutes(engine, handlers.Publications, middlewares)
	if err := setRepositoryRoutes(engine, handlers.Repositories); err != nil {
		return errors.WrapErr(err, "failed to register repo index assets")
	}
	return nil
}

func setPublicRoutes(
	api *gin.RouterGroup,
	handlers *handler.Set,
	middlewares *middleware.Middleware,
) {
	repositories := handlers.Repositories
	api.GET("/hello", handlers.System.HelloHandler)
	api.GET("/teapot", handlers.System.TeapotHandler)
	api.GET("/repos", repositories.ReposHandler)
	api.GET("/repos/:repo", repositories.RepoDetailHandler)
	api.GET("/repos/:repo/:arch/packages", repositories.AllPkgsHandler)
	api.GET("/repos/:repo/:arch/packages/:name", repositories.PkgDetailHandler)
	api.GET("/repos/:repo/:arch/packages/:name/files", repositories.PkgFilesHandler)
	api.GET("/repos/:repo/:arch/signed-url", repositories.SignedURLHandler)
	api.GET("/features", handlers.System.FeaturesHandler)
	api.POST(
		"/bug-reports",
		middlewares.RateLimit(rate.Every(2*time.Second), 3),
		handlers.BugReports.SubmitBugReportHandler,
	)
}

func setAuthRoutes(
	api *gin.RouterGroup,
	authHandlers *handler.AuthHandler,
	limit gin.HandlerFunc,
) {
	api.GET("/auth/github/login", limit, authHandlers.GitHubLoginHandler)
	api.GET("/auth/github/callback", authHandlers.GitHubCallbackHandler)
	api.GET("/auth/cli/start", limit, authHandlers.CLIStartHandler)
	api.POST("/auth/cli/exchange", limit, authHandlers.CLIExchangeHandler)
	api.GET("/auth/web/start", limit, authHandlers.WebStartHandler)
	api.POST("/auth/web/exchange", limit, authHandlers.WebExchangeHandler)
	api.POST("/auth/device/code", limit, authHandlers.DeviceCodeHandler)
	api.GET("/auth/device", limit, authHandlers.DeviceVerifyHandler)
	api.GET("/auth/device/approve", limit, authHandlers.DeviceApproveHandler)
	api.POST("/auth/device/token", limit, authHandlers.DeviceTokenHandler)
	api.GET("/auth/me", limit, authHandlers.MeHandler)
	api.POST("/auth/logout", authHandlers.LogoutHandler)
	api.POST("/auth/cli/revoke", limit, authHandlers.RevokeCLIHandler)
	api.POST("/auth/refresh", limit, authHandlers.RefreshHandler)
}
