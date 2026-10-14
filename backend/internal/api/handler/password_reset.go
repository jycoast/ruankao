package handler

import (
	"strconv"

	"software-exam/internal/application/service"
	"software-exam/pkg/response"

	"github.com/gin-gonic/gin"
)

type PasswordResetHandler struct {
	svc *service.PasswordResetService
}

func NewPasswordResetHandler(svc *service.PasswordResetService) *PasswordResetHandler {
	return &PasswordResetHandler{svc: svc}
}

// Submit POST /api/auth/password-reset —— 提交找回密码申请（无需登录）
//
// 账号不存在、被封、申请过多都返回同样的成功提示：
// 任何差异化响应都能被拿来枚举站内账号。
func (h *PasswordResetHandler) Submit(c *gin.Context) {
	var req struct {
		Account string `json:"account" binding:"required"`
		Contact string `json:"contact"`
		Reason  string `json:"reason"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请填写用户名或邮箱")
		return
	}
	if err := h.svc.Submit(req.Account, req.Contact, req.Reason, c.ClientIP()); err != nil {
		response.ServerError(c, err.Error())
		return
	}
	response.Success(c, nil)
}

// ListAll GET /api/admin/password-resets
func (h *PasswordResetHandler) ListAll(c *gin.Context) {
	page, pageSize := parsePage(c)
	// status: -1=全部 0=待处理 1=已处理 2=已忽略
	status, _ := strconv.Atoi(c.DefaultQuery("status", "-1"))

	list, total, err := h.svc.ListAll(page, pageSize, status)
	if err != nil {
		response.ServerError(c, err.Error())
		return
	}
	response.SuccessWithPage(c, total, page, pageSize, list)
}

// CountPending GET /api/admin/password-resets/pending-count
func (h *PasswordResetHandler) CountPending(c *gin.Context) {
	count, err := h.svc.CountPending()
	if err != nil {
		response.ServerError(c, err.Error())
		return
	}
	response.Success(c, gin.H{"count": count})
}

// Handle PUT /api/admin/password-resets/:id —— 通过（改密码）或忽略
func (h *PasswordResetHandler) Handle(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "无效的ID")
		return
	}
	var req struct {
		Status      int8   `json:"status"`
		NewPassword string `json:"new_password"`
		AdminNote   string `json:"admin_note"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}

	switch req.Status {
	case 1:
		if err := h.svc.Approve(id, req.NewPassword, req.AdminNote); err != nil {
			response.BadRequest(c, err.Error())
			return
		}
	case 2:
		if err := h.svc.Reject(id, req.AdminNote); err != nil {
			response.BadRequest(c, err.Error())
			return
		}
	default:
		response.BadRequest(c, "无效的状态")
		return
	}
	response.Success(c, nil)
}
