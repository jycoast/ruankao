// Package router 按 domain 拆分路由注册,每个 RegisterXxxRoutes 函数对应一个
// URL 路径段(/api/auth, /api/admin, ...),装一组 handler。
//
// 设计动机:main.go 原本 ~330 行路由代码全部扁平铺开,新增一个路由需要
// 在巨型函数里找位置、容易触发冲突。这里按 domain 拆 14 个文件,每文件
// 只装自己负责的路由,改动半径小。
//
// 装配入口 NewRouter(app):统一调用各 Register* 函数,装全局中间件
// (Cors、上传缓冲上限),返回装好的 *gin.Engine。
package router

import (
	"github.com/gin-gonic/gin"

	"software-exam/cmd/server/wire"
	"software-exam/internal/api/handler"
)

// NewRouter 装配全站路由。app 来自 cmd/server/wire.NewApp。
// 顺序：先全局中间件 → /api/health → 各 domain Register*。
func NewRouter(app *wire.App) *gin.Engine {
	r := gin.Default()
	r.Use(app.Middlewares.Cors)
	// 维护中间件必须紧跟在 Cors 之后：预检请求不带 Authorization，
	// 被维护挡下的话浏览器只会报跨域失败，而不是给用户一个维护提示。
	r.Use(app.Middlewares.Maintenance)
	// 上传走内存缓冲的上限。超出部分由 gin 落临时文件,
	// 真正的大小限制在 upload handler 里按配置卡。
	r.MaxMultipartMemory = 8 << 20

	r.GET("/api/health", handler.Health)

	// SEO：构建期预渲染的数据出口（公开内容清单）。
	// 无鉴权、无限流 —— 构建脚本抓取本来就是目标行为,限流只会掉收录。
	registerSEO(r, app.Handlers)

	// 站点级状态：/api/site-status（公开）+ /api/admin/site-settings（后台）
	registerSite(r, app.Handlers, app.Middlewares)

	registerAuth(r, app.Handlers, app.Middlewares)
	registerUser(r, app.Handlers, app.Middlewares)
	registerQuestion(r, app.Handlers, app.Middlewares)
	registerComment(r, app.Handlers, app.Middlewares)
	registerPractice(r, app.Handlers, app.Middlewares)
	registerDiagnosis(r, app.Handlers, app.Middlewares)
	registerFavorite(r, app.Handlers, app.Middlewares)
	registerKnowledge(r, app.Handlers, app.Middlewares)
	registerArticle(r, app.Handlers, app.Middlewares)
	registerExam(r, app.Handlers, app.Middlewares, app.Repos)
	registerAnnouncement(r, app.Handlers, app.Middlewares)
	registerEssay(r, app.Handlers, app.Middlewares)
	registerMembership(r, app.Handlers, app.Middlewares)
	registerPaymentNotify(r, app.Handlers, app.Middlewares)
	registerFeedback(r, app.Handlers, app.Middlewares)
	registerDaily(r, app.Handlers, app.Middlewares)
	registerAdmin(r, app.Handlers, app.Middlewares)
	registerUpload(r, app.Handlers, app.Middlewares)
	registerAI(r, app.Handlers, app.Middlewares)

	return r
}
