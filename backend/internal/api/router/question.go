package router

import (
	"github.com/gin-gonic/gin"

	"software-exam/cmd/server/wire"
)

// registerQuestion 题库域:列表/详情/随机题/分类/我的纠错/我的笔记。
// 注意:comments 路由不在这里,见 registerComment——
// 因为 comments 同时挂在 question 详情页下,原 main.go 把它拆开避免与主 group 冲突。
func registerQuestion(r *gin.Engine, h *wire.Handlers, mw *wire.Middlewares) {
	questions := r.Group("/api/questions")
	questions.Use(mw.Auth, mw.RateGlobal, mw.RateData)
	{
		questions.GET("", h.Question.List)
		questions.GET("/:id", mw.TrackQA, h.Question.GetByID)
		questions.GET("/random", h.Question.Random)
		questions.GET("/categories", h.Question.Categories)
		questions.GET("/my-reports", h.Report.Mine)
		questions.POST("/:id/report", h.Report.Submit)
		questions.GET("/:id/note", h.QuestionNote.Get)
		questions.PUT("/:id/note", h.QuestionNote.Save)
		questions.DELETE("/:id/note", h.QuestionNote.Delete)
	}
}
