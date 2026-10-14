package router

import (
	"github.com/gin-gonic/gin"

	"software-exam/cmd/server/wire"
)

// registerUpload 普通用户上传通道（笔记图片等），挂在 /api/upload，
// 仅需 Auth + 与题库相同的 RateGlobal/RateData，不要求 Admin/会员。
// admin 编辑器的图片上传仍走 registerAdmin 里的 admin.POST("/upload/image", …)，
// 此处只新增给普通用户用的入口，避免与 admin 通道互窜。
func registerUpload(r *gin.Engine, h *wire.Handlers, mw *wire.Middlewares) {
	g := r.Group("/api/upload")
	g.Use(mw.Auth, mw.RateGlobal, mw.RateData)
	{
		g.POST("/image", h.Upload.ImageForUser)
	}
}
