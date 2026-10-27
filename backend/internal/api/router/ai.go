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
		// AI 功能:非会员消耗免费试用额度。
		// 依次过限流(429 不占额度) → 在途并发 → 免费额度,任一超限都不调上游。
		gen := ai.Group("")
		gen.Use(mw.RateAI, mw.AIConcurrency, mw.AIQuota)
		{
			gen.POST("/chat", h.AI.Chat)
			gen.POST("/explain", h.AI.Explain)
			gen.POST("/explain-stream", h.AI.ExplainStream)
			gen.POST("/generate-questions", h.AI.GenerateQuestions)
			gen.POST("/generate-essay", h.AI.GenerateEssay)
			gen.POST("/grade-essay", h.AI.GradeEssay)
			gen.POST("/grade-case", h.AI.GradeCase)
		}
	}
}
