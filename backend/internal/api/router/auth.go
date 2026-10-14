package router

import (
	"github.com/gin-gonic/gin"

	"software-exam/cmd/server/wire"
	"software-exam/internal/api/handler"
)

// registerAuth 鉴权/注册/找回密码路由。
// 验证码独立分组走 rlPublic——auth 组的 rlIP 限 10/min/IP,刷新几次就被拦。
// 找回密码申请限额严:唯一未登录就能往库里写行的接口。
func registerAuth(r *gin.Engine, h *wire.Handlers, mw *wire.Middlewares) {
	auth := r.Group("/api/auth")
	auth.Use(mw.RateIP)
	{
		auth.POST("/register", h.User.Register)
		auth.POST("/login", h.User.Login)
		auth.POST("/guest", h.User.GuestLogin)
		auth.GET("/oauth/linuxdo", h.OAuth.LinuxDoInitiate)
		auth.GET("/oauth/linuxdo/callback", h.OAuth.LinuxDoCallback)
		auth.GET("/oauth/linuxdo/status", h.OAuth.LinuxDoStatus)
		auth.POST("/password-reset", mw.RateAction("password-reset", 5), h.PasswordReset.Submit)
	}

	authPublic := r.Group("/api/auth")
	authPublic.Use(mw.RatePublic)
	authPublic.GET("/captcha", handler.GetCaptcha)
}
