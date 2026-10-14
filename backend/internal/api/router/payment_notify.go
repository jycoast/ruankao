package router

import (
	"github.com/gin-gonic/gin"

	"software-exam/cmd/server/wire"
)

// registerPaymentNotify 支付异步回调,公开路由(无需登录)。
// 三条通道密钥不同,各走各的路径以便用对应密钥验签:
//   - /api/pay/notify:通用通道
//   - /api/pay/ldc/notify:LDC 积分通道
//   - /api/pay/ytpay/notify:悦通云通道
//
// 限额取得较宽松(60/min):宁可放过刷子,也不能因限流丢掉真实支付通知。
// GET 和 POST 各挂一次,兼容不同支付通道的回调方式。
func registerPaymentNotify(r *gin.Engine, h *wire.Handlers, mw *wire.Middlewares) {
	notifyRL := mw.RateAction("paynotify", 60)
	r.GET("/api/pay/notify", notifyRL, h.Payment.Notify)
	r.POST("/api/pay/notify", notifyRL, h.Payment.Notify)
	r.GET("/api/pay/ldc/notify", notifyRL, h.Payment.NotifyLDC)
	r.POST("/api/pay/ldc/notify", notifyRL, h.Payment.NotifyLDC)
	r.GET("/api/pay/ytpay/notify", notifyRL, h.Payment.NotifyYueTongYun)
	r.POST("/api/pay/ytpay/notify", notifyRL, h.Payment.NotifyYueTongYun)
}
