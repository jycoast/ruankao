package handler

import (
	"strconv"

	"software-exam/internal/application/service"
	"software-exam/pkg/response"

	"github.com/gin-gonic/gin"
)

type NotificationHandler struct {
	svc *service.NotificationService
}

func NewNotificationHandler(svc *service.NotificationService) *NotificationHandler {
	return &NotificationHandler{svc: svc}
}

// List 我的通知 GET /api/notifications?page&page_size&unread=1
func (h *NotificationHandler) List(c *gin.Context) {
	userID := c.GetUint64("user_id")
	page, pageSize := parsePage(c)
	onlyUnread := c.Query("unread") == "1"

	list, total, err := h.svc.MyList(userID, page, pageSize, onlyUnread)
	if err != nil {
		response.ServerError(c, err.Error())
		return
	}
	response.SuccessWithPage(c, total, page, pageSize, list)
}

// UnreadCount 未读数 GET /api/notifications/unread-count
func (h *NotificationHandler) UnreadCount(c *gin.Context) {
	userID := c.GetUint64("user_id")

	count, err := h.svc.UnreadCount(userID)
	if err != nil {
		response.ServerError(c, err.Error())
		return
	}
	response.Success(c, gin.H{"count": count})
}

// MarkRead 标记单条已读 PUT /api/notifications/:id/read
func (h *NotificationHandler) MarkRead(c *gin.Context) {
	userID := c.GetUint64("user_id")
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "无效的ID")
		return
	}

	// userID 一并传下去，越权由 SQL 条件挡掉
	if err := h.svc.MarkRead(id, userID); err != nil {
		response.ServerError(c, err.Error())
		return
	}
	response.Success(c, nil)
}

// MarkAllRead 全部已读 PUT /api/notifications/read-all
func (h *NotificationHandler) MarkAllRead(c *gin.Context) {
	userID := c.GetUint64("user_id")

	if err := h.svc.MarkAllRead(userID); err != nil {
		response.ServerError(c, err.Error())
		return
	}
	response.Success(c, nil)
}
