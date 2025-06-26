package handler

import (
	"github.com/Hayao0819/Kamisato/ayato/service"
	"github.com/Hayao0819/Kamisato/internal/conf"
	"github.com/gin-gonic/gin"
)

// 最終的にServiceのみの依存にする
type Handler struct {
	cfg *conf.AyatoConfig // 間違った依存なので/いつか消す -> 別に良いらしい？
	s   service.IService
}

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

func New(service service.IService, cfg *conf.AyatoConfig) IHandler {
	return &Handler{
		s:   service,
		cfg: cfg,
	}
}
