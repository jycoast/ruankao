package router

import (
	"github.com/gin-gonic/gin"

	"software-exam/cmd/server/wire"
)

// registerAdmin 后台管理路由。所有路径前缀 /api/admin/*,统一挂 authMW + AdminMiddleware。
//
// 涵盖:题库 CRUD + 排序 / 用户管理 / 仪表盘 / 在线用户 / 公告 / 纠错 / 反馈 /
// 找回密码 / 邀请码 / AI 配置 / 会员授予 / 订单 / 套餐 / 卡密 / 试卷管理 /
// 反爬嫌疑 / 上传图片 / 评论管理 / 评论举报 / 考试排期。
//
// 注意:knowledge/article/essay 的后台路由在各自 registerXxx 里单独挂,
// 因为它们的 URL 前缀不是 /api/admin/* 而是 /api/admin/knowledge 等,
// 保持 prefix 与公开路由对齐以便 SEO/SEO 测试。
func registerAdmin(r *gin.Engine, h *wire.Handlers, mw *wire.Middlewares) {
	admin := r.Group("/api/admin")
	admin.Use(mw.Auth, mw.Admin)
	{
		// 题库 CRUD + 排序
		admin.POST("/questions", h.Question.Create)
		admin.POST("/questions/sort", h.Question.Sort)
		admin.PUT("/questions/:id", h.Question.Update)
		admin.DELETE("/questions/:id", h.Question.Delete)

		// 用户管理 + 仪表盘 + 在线
		admin.GET("/users", h.Admin.ListUsers)
		admin.PUT("/users/:id/status", h.Admin.UpdateUserStatus)
		admin.PUT("/users/:id/role", h.Admin.UpdateUserRole)
		admin.PUT("/users/:id/password", h.Admin.ResetUserPassword)
		admin.GET("/dashboard", h.Admin.GetDashboard)
		admin.GET("/online", h.Admin.GetOnline)

		// 公告 CRUD
		admin.GET("/announcements", h.Announcement.ListAll)
		admin.POST("/announcements", h.Announcement.Create)
		admin.PUT("/announcements/:id", h.Announcement.Update)
		admin.DELETE("/announcements/:id", h.Announcement.Delete)

		// 题目纠错 + 待处理计数
		admin.GET("/reports", h.Report.ListAll)
		admin.PUT("/reports/:id", h.Report.UpdateStatus)
		admin.GET("/reports/pending-count", h.Report.CountPending)

		// 反馈 + 待处理计数
		admin.GET("/feedbacks", h.Feedback.ListAll)
		admin.PUT("/feedbacks/:id", h.Feedback.UpdateStatus)
		admin.GET("/feedbacks/pending-count", h.Feedback.CountPending)

		// 找回密码审批
		admin.GET("/password-resets", h.PasswordReset.ListAll)
		admin.PUT("/password-resets/:id", h.PasswordReset.Handle)
		admin.GET("/password-resets/pending-count", h.PasswordReset.CountPending)

		// 邀请码
		admin.POST("/invite-codes/generate", h.InviteCode.AdminGenerate)
		admin.GET("/invite-codes", h.InviteCode.AdminList)
		admin.DELETE("/invite-codes/:id", h.InviteCode.AdminDelete)

		// AI 配置 + 调用日志
		admin.GET("/ai/config", h.AI.GetConfig)
		admin.PUT("/ai/config", h.AI.UpdateConfig)
		admin.POST("/ai/test", h.AI.TestConfig)
		admin.GET("/ai/usage", h.AI.Usage)
		admin.GET("/ai/logs", h.AI.ListCallLogs)

		// 会员授予 + 订单 + 套餐
		admin.POST("/memberships/grant", h.Payment.AdminGrant)
		admin.GET("/orders", h.Payment.AdminListOrders)
		admin.GET("/membership/plans", h.Payment.AdminListPlans)
		admin.PUT("/membership/plans/:key", h.Payment.AdminUpdatePlan)

		// 卡密生成 + 列表
		admin.POST("/card-keys/generate", h.CardKey.AdminGenerate)
		admin.GET("/card-keys", h.CardKey.AdminList)
		admin.DELETE("/card-keys/:id", h.CardKey.AdminDelete)

		// 试卷管理(含卷内题目编排)
		admin.GET("/papers", h.Exam.AdminListPapers)
		admin.POST("/papers", h.Exam.AdminCreatePaper)
		admin.PUT("/papers/:id", h.Exam.AdminUpdatePaper)
		admin.DELETE("/papers/:id", h.Exam.AdminDeletePaper)
		admin.GET("/papers/:id/questions", h.Exam.AdminGetPaperQuestions)
		admin.PUT("/papers/:id/questions", h.Exam.AdminReplacePaperQuestions)

		// 反爬嫌疑管理
		admin.GET("/anticrawl/suspects", h.AntiCrawl.ListSuspects)
		admin.DELETE("/anticrawl/suspects/:user_id", h.AntiCrawl.RemoveSuspect)

		// 后台编辑器粘贴上传图片
		admin.POST("/upload/image", h.Upload.Image)

		// 评论管理
		admin.GET("/comments", h.Comment.AdminList)
		admin.PUT("/comments/:id/status", h.Comment.AdminSetStatus)
		admin.GET("/comment-reports", h.Comment.AdminReportList)
		admin.PUT("/comment-reports/:id", h.Comment.AdminHandleReport)

		// 考试排期(全局配置)
		admin.GET("/exam-schedules", h.ExamSchedule.List)
		admin.POST("/exam-schedules", h.ExamSchedule.Create)
		admin.PUT("/exam-schedules/:id", h.ExamSchedule.Update)
		admin.DELETE("/exam-schedules/:id", h.ExamSchedule.Delete)
	}
}
