package handler

import (
	"software-exam/internal/application/service"
	"software-exam/pkg/response"
	"strconv"

	"github.com/gin-gonic/gin"
)

type InviteCodeHandler struct {
	svc *service.InviteCodeService
}

func NewInviteCodeHandler(svc *service.InviteCodeService) *InviteCodeHandler {
	return &InviteCodeHandler{svc: svc}
}

// AdminGenerate POST /api/admin/invite-codes/generate
func (h *InviteCodeHandler) AdminGenerate(c *gin.Context) {
	var req struct {
		Count int    `json:"count" binding:"required,min=1,max=100"`
		Note  string `json:"note"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误，数量需在1-100之间")
		return
	}
	createdBy := c.GetUint64("user_id")
	codes, err := h.svc.Generate(req.Count, createdBy, req.Note)
	if err != nil {
		response.ServerError(c, err.Error())
		return
	}
	response.Success(c, gin.H{"codes": codes})
}

// AdminList GET /api/admin/invite-codes
func (h *InviteCodeHandler) AdminList(c *gin.Context) {
	page, pageSize := parsePage(c)
	list, total, err := h.svc.List(page, pageSize)
	if err != nil {
		response.ServerError(c, err.Error())
		return
	}
	response.SuccessWithPage(c, total, page, pageSize, list)
}

// AdminDelete DELETE /api/admin/invite-codes/:id  (只能删未使用的)
func (h *InviteCodeHandler) AdminDelete(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "无效的ID")
		return
	}
	if err := h.svc.Delete(id); err != nil {
		response.ServerError(c, err.Error())
		return
	}
	response.Success(c, nil)
}
