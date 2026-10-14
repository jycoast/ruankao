package router

import (
	"github.com/gin-gonic/gin"

	"software-exam/cmd/server/wire"
)

// registerArticle 范文/写作模板域,与知识点一样的「公开引流」策略。
func registerArticle(r *gin.Engine, h *wire.Handlers, mw *wire.Middlewares) {
	articles := r.Group("/api/articles")
	articles.Use(mw.OptionalAuth, mw.RatePublic)
	{
		articles.GET("", h.Article.List)
		articles.GET("/:id", h.Article.GetByID)
	}

	articleAdmin := r.Group("/api/admin/articles")
	articleAdmin.Use(mw.Auth, mw.Admin)
	{
		articleAdmin.POST("", h.Article.Create)
		articleAdmin.PUT("/:id", h.Article.Update)
		articleAdmin.DELETE("/:id", h.Article.Delete)
	}
}
