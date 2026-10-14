package router

import (
	"github.com/gin-gonic/gin"

	"software-exam/cmd/server/wire"
)

// registerMembership 会员与卡密:
//   - 套餐/订单/卡密兑换全部已登录
//   - 卡密可被穷举,限额比其它接口严格得多(RateAction "redeem" 5/min)
func registerMembership(r *gin.Engine, h *wire.Handlers, mw *wire.Middlewares) {
	membershipGroup := r.Group("/api/membership")
	membershipGroup.Use(mw.Auth, mw.RateGlobal)
	{
		membershipGroup.GET("/plans", h.Payment.Plans)
		membershipGroup.GET("/channels", h.Payment.PayChannels)
		membershipGroup.GET("/status", h.Payment.Status)
		membershipGroup.POST("/orders", mw.RateAction("order", 10), h.Payment.CreateOrder)
		membershipGroup.GET("/orders", h.Payment.ListOrders)
		membershipGroup.GET("/orders/:out_trade_no", h.Payment.QueryOrder)
		membershipGroup.POST("/redeem", mw.RateAction("redeem", 5), h.CardKey.Redeem)
	}
}
