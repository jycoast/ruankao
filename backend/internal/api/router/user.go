package router

import (
	"github.com/gin-gonic/gin"

	"software-exam/cmd/server/wire"
)

// registerAuthUser 用户域已登录路由:个人资料、密码、统计、报告。
// /api/my/notes 和 /api/notifications 也归这里,因为它们都是「我的」数据。
func registerUser(r *gin.Engine, h *wire.Handlers, mw *wire.Middlewares) {
	user := r.Group("/api/user")
	user.Use(mw.Auth, mw.RateGlobal)
	{
		user.GET("/profile", h.User.GetProfile)
		user.PUT("/profile", h.User.UpdateProfile)
		user.PUT("/subject", h.User.UpdateSubject)
		user.GET("/stats", h.User.GetStats)
		user.GET("/report", h.Practice.GetReport)
		user.PUT("/password", h.User.ChangePassword)
	}

	myNotes := r.Group("/api/my")
	myNotes.Use(mw.Auth, mw.RateGlobal)
	myNotes.GET("/notes", h.QuestionNote.MyNotes)
	// 详情必须挂在 /notes 之后注册。参数名是 :question_id（不是 :id），见 handler 的注释。
	myNotes.GET("/notes/:question_id", h.QuestionNote.Detail)

	notifications := r.Group("/api/notifications")
	notifications.Use(mw.Auth, mw.RateGlobal)
	{
		notifications.GET("", h.Notification.List)
		notifications.GET("/unread-count", h.Notification.UnreadCount)
		notifications.PUT("/read-all", h.Notification.MarkAllRead)
		notifications.PUT("/:id/read", h.Notification.MarkRead)
	}
}
