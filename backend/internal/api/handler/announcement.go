package handler

import (
	"software-exam/internal/application/service"
	"software-exam/pkg/response"
	"strconv"

	"github.com/gin-gonic/gin"
)

type AnnouncementHandler struct {
	svc *service.AnnouncementService
}

func NewAnnouncementHandler(svc *service.AnnouncementService) *AnnouncementHandler {
	return &AnnouncementHandler{svc: svc}
}

// ---- 用户端 ----

// ListUnread 返回当前用户未读的公告
func (h *AnnouncementHandler) ListUnread(c *gin.Context) {
	userID := c.GetUint64("user_id")
	list, err := h.svc.ListUnread(userID)
	if err != nil {
		response.ServerError(c, err.Error())
		return
	}
	response.Success(c, list)
}

// MarkRead 标记某条公告为已读
func (h *AnnouncementHandler) MarkRead(c *gin.Context) {
	userID := c.GetUint64("user_id")
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "无效的ID")
		return
	}
	if err := h.svc.MarkRead(userID, id); err != nil {
		response.ServerError(c, err.Error())
		return
	}
	response.Success(c, nil)
}

// ---- 管理端 ----

func (h *AnnouncementHandler) ListAll(c *gin.Context) {
	page, pageSize := parsePage(c)
	list, total, err := h.svc.ListAll(page, pageSize)
	if err != nil {
		response.ServerError(c, err.Error())
		return
	}
	response.SuccessWithPage(c, total, page, pageSize, list)
}

func (h *AnnouncementHandler) Create(c *gin.Context) {
	var req struct {
		Title   string `json:"title" binding:"required"`
		Content string `json:"content" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}
	createdBy := c.GetUint64("user_id")
	id, err := h.svc.Create(req.Title, req.Content, createdBy)
	if err != nil {
		response.ServerError(c, err.Error())
		return
	}
	response.Success(c, gin.H{"id": id})
}

func (h *AnnouncementHandler) Update(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "无效的ID")
		return
	}
	var req struct {
		Title   string `json:"title"`
		Content string `json:"content"`
		Status  int8   `json:"status"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}
	if err := h.svc.Update(id, req.Title, req.Content, req.Status); err != nil {
		response.ServerError(c, err.Error())
		return
	}
	response.Success(c, nil)
}

func (h *AnnouncementHandler) Delete(c *gin.Context) {
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
