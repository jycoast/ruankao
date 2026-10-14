package router

import (
	"github.com/gin-gonic/gin"

	"software-exam/cmd/server/wire"
)

// registerComment 评论区:
//   - 列表游客可看(optionalAuth),挂在 questions group 下
//   - 写操作需登录,在 /api group 下独立挂(避免再次走 questions.Use 的限流)
//
// 原 main.go 行 213-226 把这两块拆开是因为 questions.Use 里挂了更严格的
// rlData 限流,游客看评论不该触发。保持拆分语义。
func registerComment(r *gin.Engine, h *wire.Handlers, mw *wire.Middlewares) {
	commentGuest := r.Group("/api/questions")
	commentGuest.Use(mw.OptionalAuth, mw.RatePublic)
	commentGuest.GET("/:id/comments", h.Comment.List)

	commentUser := r.Group("/api")
	commentUser.Use(mw.Auth, mw.RateGlobal)
	{
		commentUser.POST("/questions/:id/comments", mw.RateAction("comment", 12), h.Comment.Create)
		commentUser.DELETE("/comments/:id", h.Comment.Delete)
		commentUser.POST("/comments/:id/report", mw.RateAction("comment_report", 10), h.Comment.Report)
	}
}
