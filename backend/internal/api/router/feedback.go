package router

import (
	"github.com/gin-gonic/gin"

	"software-exam/cmd/server/wire"
)

// registerFeedback 用户反馈 / 建议。后台审核流走 admin group。
func registerFeedback(r *gin.Engine, h *wire.Handlers, mw *wire.Middlewares) {
	feedbackGroup := r.Group("/api/feedbacks")
	feedbackGroup.Use(mw.Auth, mw.RateGlobal)
	{
		feedbackGroup.POST("", h.Feedback.Submit)
		feedbackGroup.GET("/mine", h.Feedback.Mine)
	}
}
