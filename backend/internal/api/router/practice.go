package router

import (
	"github.com/gin-gonic/gin"

	"software-exam/cmd/server/wire"
)

// registerPractice 练习域:答题、错题本、复习(会员)、历史、分类进度。
// /review 是会员专属功能,挂 premium 中间件。
func registerPractice(r *gin.Engine, h *wire.Handlers, mw *wire.Middlewares) {
	practice := r.Group("/api/practice")
	practice.Use(mw.Auth, mw.RateGlobal)
	{
		practice.POST("/submit", h.Practice.SubmitAnswer)
		practice.POST("/start", h.Practice.StartPractice)
		practice.GET("/session", h.Practice.GetSessionState)
		practice.GET("/wrong", h.Practice.GetWrongQuestions)
		practice.DELETE("/wrong/:question_id", h.Practice.DeleteWrongQuestion)
		practice.GET("/review", mw.RequireMember, h.Practice.GetDueReview)
		practice.POST("/review", mw.RequireMember, h.Practice.Review)
		practice.GET("/history", h.Practice.GetHistory)
		practice.GET("/category-progress", h.Practice.GetCategoryProgress)
	}
}
