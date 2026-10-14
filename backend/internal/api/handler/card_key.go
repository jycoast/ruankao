package handler

import (
	"software-exam/internal/application/service"
	"software-exam/internal/domain/repository"
	"software-exam/pkg/response"
	"strconv"

	"github.com/gin-gonic/gin"
)

type CardKeyHandler struct {
	svc *service.CardKeyService
}

func NewCardKeyHandler(svc *service.CardKeyService) *CardKeyHandler {
	return &CardKeyHandler{svc: svc}
}

// AdminGenerate POST /api/admin/card-keys/generate
func (h *CardKeyHandler) AdminGenerate(c *gin.Context) {
	var req struct {
		Count int    `json:"count" binding:"required,min=1,max=100"`
		Plan  string `json:"plan" binding:"required"`
		Note  string `json:"note"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误，数量需在1-100之间且须选择套餐")
		return
	}
	createdBy := c.GetUint64("user_id")
	codes, err := h.svc.Generate(req.Count, req.Plan, createdBy, req.Note)
	if err != nil {
		response.ServerError(c, err.Error())
		return
	}
	response.Success(c, gin.H{"codes": codes})
}

// AdminList GET /api/admin/card-keys
func (h *CardKeyHandler) AdminList(c *gin.Context) {
	page, pageSize := parsePage(c)
	status := int8(-1)
	if v := c.Query("status"); v != "" {
		if n, err := strconv.ParseInt(v, 10, 8); err == nil {
			status = int8(n)
		}
	}
	req := repository.CardKeyListReq{
		Page:     page,
		PageSize: pageSize,
		Plan:     c.Query("plan"),
		Status:   status,
		Keyword:  c.Query("keyword"),
	}
	list, total, err := h.svc.List(req)
	if err != nil {
		response.ServerError(c, err.Error())
		return
	}
	response.SuccessWithPage(c, total, page, pageSize, list)
}

// AdminDelete DELETE /api/admin/card-keys/:id  (只能删未使用的)
func (h *CardKeyHandler) AdminDelete(c *gin.Context) {
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

// Redeem POST /api/membership/redeem —— 用户使用卡密兑换会员
func (h *CardKeyHandler) Redeem(c *gin.Context) {
	var req struct {
		Code string `json:"code" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请输入卡密")
		return
	}
	userID := c.GetUint64("user_id")
	planName, days, err := h.svc.Redeem(req.Code, userID)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, gin.H{"plan": planName, "days": days})
}
