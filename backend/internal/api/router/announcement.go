package router

import (
	"github.com/gin-gonic/gin"

	"software-exam/cmd/server/wire"
)

// registerAnnouncement 已登录用户看公告:未读列表 + 标记已读。
// 后台公告 CRUD 在 registerAdmin 里。
func registerAnnouncement(r *gin.Engine, h *wire.Handlers, mw *wire.Middlewares) {
	announcements := r.Group("/api/announcements")
	announcements.Use(mw.Auth)
	{
		announcements.GET("", h.Announcement.ListUnread)
		announcements.POST("/:id/read", h.Announcement.MarkRead)
	}
}
