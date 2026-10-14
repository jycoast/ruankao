package router

import (
	"github.com/gin-gonic/gin"

	"software-exam/cmd/server/wire"
)

// registerDiagnosis 诊断报告域（会员专属页 /my/diagnosis）。
//
// 整组会员专属：数据端点 + AI 端点都挂 RequireMember，不给免费试用
// （不挂 AIQuota —— 诊断不消耗免费额度，免费用户看数据引导页即可）。
// 管理员账号在 RequireMember 内自动放行。
func registerDiagnosis(r *gin.Engine, h *wire.Handlers, mw *wire.Middlewares) {
	d := r.Group("/api/diagnosis")
	d.Use(mw.Auth, mw.RateGlobal, mw.RequireMember)
	{
		d.GET("/summary", h.Practice.GetDiagnosisSummary)
		d.POST("/ai", h.AI.DiagnosisStream)
	}
}
