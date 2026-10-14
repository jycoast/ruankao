package handler

import (
	"software-exam/internal/application/service"
	"software-exam/pkg/response"

	"github.com/gin-gonic/gin"
)

// SiteHandler 站点级开关的 HTTP 入口。
type SiteHandler struct {
	siteService *service.SiteService
}

func NewSiteHandler(siteService *service.SiteService) *SiteHandler {
	return &SiteHandler{siteService: siteService}
}

// Status GET /api/site-status（公开）
//
// 前端路由守卫读它来决定要不要改道维护页。公开是因为维护页本身
// 就得能被未登录用户看到 —— 这也是它必须进维护白名单的原因。
func (h *SiteHandler) Status(c *gin.Context) {
	response.Success(c, gin.H{"maintenance": h.siteService.MaintenanceEnabled(c.Request.Context())})
}

// GetSettings GET /api/admin/site-settings
//
// 走和中间件同一个读取路径（含缓存），刻意不回源查库：
// 后台显示什么、实际执行的就是什么，不会出现「后台显示已关闭、请求仍被挡」的错位。
func (h *SiteHandler) GetSettings(c *gin.Context) {
	response.Success(c, gin.H{"maintenance_enabled": h.siteService.MaintenanceEnabled(c.Request.Context())})
}

// UpdateSettings PUT /api/admin/site-settings
//
// MaintenanceEnabled 用指针：普通 bool 遇到 body 里没有这个字段时会绑成
// false，于是 POST 一个 {} 就把维护静默关掉了 —— 一个写错的请求体不该
// 有这种效果。缺字段直接 400，宁可报错也不要静默改状态。
func (h *SiteHandler) UpdateSettings(c *gin.Context) {
	var req struct {
		MaintenanceEnabled *bool `json:"maintenance_enabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.MaintenanceEnabled == nil {
		response.BadRequest(c, "参数错误")
		return
	}
	if err := h.siteService.SetMaintenance(*req.MaintenanceEnabled); err != nil {
		// 保存失败必须让管理员看到：以为开上了其实没开，是最糟的结果
		response.ServerError(c, "保存失败："+err.Error())
		return
	}
	response.Success(c, gin.H{"maintenance_enabled": *req.MaintenanceEnabled})
}
