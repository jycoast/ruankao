package handler

import (
	"software-exam/internal/application/service"
	"software-exam/pkg/response"
	"strconv"

	"github.com/gin-gonic/gin"
)

type AdminHandler struct {
	adminService *service.AdminService
}

func NewAdminHandler(adminService *service.AdminService) *AdminHandler {
	return &AdminHandler{adminService: adminService}
}

func (h *AdminHandler) ListUsers(c *gin.Context) {
	page, pageSize := parsePage(c)
	keyword := c.Query("keyword")
	role := int8(-1)
	if v := c.Query("role"); v != "" {
		if n, err := strconv.ParseInt(v, 10, 8); err == nil {
			role = int8(n)
		}
	}
	status := int8(-1)
	if v := c.Query("status"); v != "" {
		if n, err := strconv.ParseInt(v, 10, 8); err == nil {
			status = int8(n)
		}
	}
	users, total, err := h.adminService.ListUsers(page, pageSize, keyword, role, status)
	if err != nil {
		response.ServerError(c, err.Error())
		return
	}
	response.SuccessWithPage(c, total, page, pageSize, users)
}

func (h *AdminHandler) UpdateUserStatus(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "无效的ID")
		return
	}
	var req struct {
		Status int8 `json:"status"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}
	if err := h.adminService.UpdateUserStatus(id, req.Status); err != nil {
		response.ServerError(c, err.Error())
		return
	}
	response.Success(c, nil)
}

func (h *AdminHandler) UpdateUserRole(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "无效的ID")
		return
	}
	var req struct {
		Role int8 `json:"role"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}
	if req.Role != 0 && req.Role != 1 {
		response.BadRequest(c, "无效的角色")
		return
	}
	if err := h.adminService.UpdateUserRole(id, req.Role); err != nil {
		response.ServerError(c, err.Error())
		return
	}
	response.Success(c, nil)
}

func (h *AdminHandler) ResetUserPassword(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "无效的ID")
		return
	}
	var req struct {
		NewPassword string `json:"new_password" binding:"required,min=6"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "新密码长度不能少于6位")
		return
	}
	if err := h.adminService.ResetUserPassword(id, req.NewPassword); err != nil {
		response.Error(c, 400, err.Error())
		return
	}
	response.Success(c, nil)
}

func (h *AdminHandler) GetDashboard(c *gin.Context) {
	stats, err := h.adminService.GetDashboard()
	if err != nil {
		response.ServerError(c, err.Error())
		return
	}
	response.Success(c, stats)
}

// GetOnline 在线用户统计（登录用户 N 分钟内有活跃请求的数量 + 最近活跃列表）。
func (h *AdminHandler) GetOnline(c *gin.Context) {
	stats, err := h.adminService.GetOnline()
	if err != nil {
		response.ServerError(c, err.Error())
		return
	}
	response.Success(c, stats)
}
