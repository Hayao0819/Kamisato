package handler

import (
	"github.com/Hayao0819/Kamisato/ayato/service"
	"github.com/Hayao0819/Kamisato/internal/conf"
	"github.com/gin-gonic/gin"
)

// Handler is a struct that handles API requests.
// Eventually planned to depend only on Service.
type Handler struct {
	cfg *conf.AyatoConfig // Configuration (planned to reduce dependency in the future)
	s   service.IService  // Service layer
}

// IHandler is the interface for API handlers.
type IHandler interface {
	HelloHandler(ctx *gin.Context)
	TeapotHandler(ctx *gin.Context)
	BlinkyUploadHandler(ctx *gin.Context)
	ReposHandler(ctx *gin.Context)
	ArchesHandler(ctx *gin.Context)
	AllPkgsHandler(ctx *gin.Context)
	PkgDetailHandler(ctx *gin.Context)
	PkgFilesHandler(ctx *gin.Context)
	PkgDetailFile(ctx *gin.Context)
	RepoFileHandler(ctx *gin.Context)
	RepoFileListHandler(ctx *gin.Context)
	BlinkyRemoveHandler(ctx *gin.Context)
}

// New is the constructor for Handler.
func New(service service.IService, cfg *conf.AyatoConfig) IHandler {
	return &Handler{
		s:   service,
		cfg: cfg,
	}
}
