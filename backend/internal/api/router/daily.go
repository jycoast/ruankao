package router

import (
	"github.com/gin-gonic/gin"

	"software-exam/cmd/server/wire"
)

// registerDaily 每日一练 / 签到。Session 用于断点续练。
func registerDaily(r *gin.Engine, h *wire.Handlers, mw *wire.Middlewares) {
	daily := r.Group("/api/daily")
	daily.Use(mw.Auth, mw.RateGlobal)
	{
		daily.GET("/today", h.Daily.Today)
		daily.POST("/checkin", h.Daily.Checkin)
		daily.POST("/session", h.Daily.SaveSession)
	}
}
