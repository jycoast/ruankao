package router

import (
	"github.com/gin-gonic/gin"

	"software-exam/cmd/server/wire"
)

// registerEssay 论文提交与后台审核。
// 提交 + 我的提交是已登录用户的自助操作;后台审核列表/详情/打分在 admin group 下。
func registerEssay(r *gin.Engine, h *wire.Handlers, mw *wire.Middlewares) {
	essays := r.Group("/api/essays")
	essays.Use(mw.Auth)
	{
		essays.POST("/submit", h.EssaySubmission.Submit)
		essays.GET("/my-submissions", h.EssaySubmission.MySubmissions)
	}

	essayAdmin := r.Group("/api/admin/essay-submissions")
	essayAdmin.Use(mw.Auth, mw.Admin)
	{
		essayAdmin.GET("", h.EssaySubmission.AdminList)
		essayAdmin.GET("/:id", h.EssaySubmission.AdminGetOne)
		essayAdmin.PUT("/:id/review", h.EssaySubmission.AdminReview)
	}
}
