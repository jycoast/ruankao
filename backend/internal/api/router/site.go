package router

import (
	"github.com/gin-gonic/gin"

	"software-exam/cmd/server/wire"
)

// registerSite 站点级状态路由。
//
// 单独一个文件承载这个功能的全部路由，改动和 handler 一起走。
// 注意：/api/admin 前缀在 admin.go 里已有一个 Group 且挂了 Auth+Admin；
// 这里再开一个 Group 走同样的两层，gin 会把两组并进同一棵树，行为一致。
// 代价是 admin.go 那个组的中间件不再自动覆盖这里 —— 以后往 admin.go
// 的组上加中间件时，记得同步这个文件。
func registerSite(r *gin.Engine, h *wire.Handlers, mw *wire.Middlewares) {
	// 公开：前端守卫与维护页都要能读到，不带鉴权
	r.GET("/api/site-status", h.Site.Status)

	admin := r.Group("/api/admin")
	admin.Use(mw.Auth, mw.Admin)
	{
		admin.GET("/site-settings", h.Site.GetSettings)
		admin.PUT("/site-settings", h.Site.UpdateSettings)
	}
}
