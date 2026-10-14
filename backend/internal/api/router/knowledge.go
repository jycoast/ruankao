package router

import (
	"github.com/gin-gonic/gin"

	"software-exam/cmd/server/wire"
)

// registerKnowledge 知识点域:
//   - 公开组:游客可访问(全站唯一可被搜索引擎收录的长内容)
//   - 后台组:整树管理 + Reorder,page_size 放宽到 1000 防截断快照
//
// 公开组用 optionalAuth 而不是 authMW——带 token 就认身份,不带也放行,
// 配合 rlPublic(限制宽松,不影响 SEO 爬虫)。
func registerKnowledge(r *gin.Engine, h *wire.Handlers, mw *wire.Middlewares) {
	knowledge := r.Group("/api/knowledge")
	knowledge.Use(mw.OptionalAuth, mw.RatePublic)
	{
		knowledge.GET("", h.Knowledge.List)
		knowledge.GET("/:id", h.Knowledge.GetByID)
		knowledge.GET("/:id/questions", h.Knowledge.RelatedQuestions)
	}

	knowledgeAdmin := r.Group("/api/admin/knowledge")
	knowledgeAdmin.Use(mw.Auth, mw.Admin)
	{
		knowledgeAdmin.GET("", h.Knowledge.ListAdmin)
		knowledgeAdmin.POST("", h.Knowledge.Create)
		knowledgeAdmin.PUT("/reorder", h.Knowledge.Reorder)
		knowledgeAdmin.PUT("/:id", h.Knowledge.Update)
		knowledgeAdmin.DELETE("/:id", h.Knowledge.Delete)
	}
}
