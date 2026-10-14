package router

import (
	"github.com/gin-gonic/gin"

	"software-exam/cmd/server/wire"
)

// registerFavorite 收藏域:题/知识点/范文三种 target_type 共用。
// /check 是幂等查询,前端用来判断「是否已收藏」决定按钮状态。
func registerFavorite(r *gin.Engine, h *wire.Handlers, mw *wire.Middlewares) {
	favorites := r.Group("/api/favorites")
	favorites.Use(mw.Auth, mw.RateGlobal)
	{
		favorites.POST("", h.Favorite.Add)
		favorites.DELETE("/:target_type/:target_id", h.Favorite.Remove)
		favorites.GET("", h.Favorite.List)
		favorites.GET("/check", h.Favorite.Check)
	}
}
