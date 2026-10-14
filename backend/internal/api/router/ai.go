package router

import (
	"github.com/gin-gonic/gin"

	"software-exam/cmd/server/wire"
)

// registerAI AI 助手域:
//   - 会话 CRUD:已登录
//   - 我的自定义配置:会员专属(premium)
//   - AI 调用(chat/explain/...):非会员消耗免费试用额度(aiQuota)
//
// aiQuota 在 middleware 层处理会员/试用额度判定,handler 不重复卡。
func registerAI(r *gin.Engine, h *wire.Handlers, mw *wire.Middlewares) {
	ai := r.Group("/api/ai")
	ai.Use(mw.Auth, mw.RateGlobal)
	{
		ai.GET("/status", h.AI.Status)
		ai.GET("/conversations", h.AI.ListConversations)
		ai.POST("/conversations", h.AI.CreateConversation)
		ai.GET("/conversations/:id", h.AI.GetConversation)
		ai.DELETE("/conversations/:id", h.AI.DeleteConversation)
		// 会员自定义 AI 配置
		ai.GET("/my-config", mw.RequireMember, h.AI.GetMyConfig)
		ai.PUT("/my-config", mw.RequireMember, h.AI.UpdateMyConfig)
		ai.POST("/my-config/test", mw.RequireMember, h.AI.TestMyConfig)
		// AI 功能:非会员消耗免费试用额度
		ai.POST("/chat", mw.AIQuota, h.AI.Chat)
		ai.POST("/explain", mw.AIQuota, h.AI.Explain)
		ai.POST("/explain-stream", mw.AIQuota, h.AI.ExplainStream)
		ai.POST("/generate-questions", mw.AIQuota, h.AI.GenerateQuestions)
		ai.POST("/generate-essay", mw.AIQuota, h.AI.GenerateEssay)
		ai.POST("/grade-essay", mw.AIQuota, h.AI.GradeEssay)
		ai.POST("/grade-case", mw.AIQuota, h.AI.GradeCase)
	}
}
