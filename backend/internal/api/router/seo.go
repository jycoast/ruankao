package router

import (
	"github.com/gin-gonic/gin"

	"software-exam/cmd/server/wire"
)

// registerSEO 公开 SEO 路由:无鉴权、无限流,构建脚本抓取是目标行为。
func registerSEO(r *gin.Engine, h *wire.Handlers) {
	r.GET("/api/prerender/manifest", h.SEO.Manifest)
}
