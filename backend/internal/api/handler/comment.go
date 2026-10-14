package handler

import (
	"strconv"

	"software-exam/internal/application/service"
	"software-exam/pkg/response"

	"github.com/gin-gonic/gin"
)

type CommentHandler struct {
	svc *service.CommentService
}

func NewCommentHandler(svc *service.CommentService) *CommentHandler {
	return &CommentHandler{svc: svc}
}

// List GET /api/questions/:id/comments —— 游客可看(optionalAuth,未登录 user_id=0)
func (h *CommentHandler) List(c *gin.Context) {
	questionID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "无效的题目ID")
		return
	}
	page, pageSize := parsePage(c)
	lst, err := h.svc.List(questionID, page, pageSize, c.GetUint64("user_id"))
	if err != nil {
		response.ServerError(c, err.Error())
		return
	}
	response.SuccessWithPage(c, lst.Total, page, pageSize, lst.Items)
}

// Create POST /api/questions/:id/comments —— body: {content, parent_id?}
func (h *CommentHandler) Create(c *gin.Context) {
	userID := c.GetUint64("user_id")
	questionID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "无效的题目ID")
		return
	}
	var req struct {
		Content  string `json:"content"`
		ParentID uint64 `json:"parent_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求参数错误")
		return
	}
	if err := h.svc.Create(questionID, userID, req.Content, req.ParentID); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, nil)
}

// Delete DELETE /api/comments/:id
func (h *CommentHandler) Delete(c *gin.Context) {
	userID := c.GetUint64("user_id")
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "无效的评论ID")
		return
	}
	if err := h.svc.Delete(id, userID); err != nil {
		response.Forbidden(c, err.Error())
		return
	}
	response.Success(c, nil)
}

// Report POST /api/comments/:id/report —— body: {reason}
func (h *CommentHandler) Report(c *gin.Context) {
	userID := c.GetUint64("user_id")
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "无效的评论ID")
		return
	}
	var req struct {
		Reason string `json:"reason"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请选择举报理由")
		return
	}
	if err := h.svc.Report(id, userID, req.Reason); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, nil)
}

// AdminList GET /api/admin/comments
func (h *CommentHandler) AdminList(c *gin.Context) {
	status, _ := strconv.Atoi(c.DefaultQuery("status", "0"))
	questionID, _ := strconv.ParseUint(c.DefaultQuery("question_id", "0"), 10, 64)
	page, pageSize := parsePage(c)
	list, total, err := h.svc.AdminList(status, c.Query("keyword"), questionID, page, pageSize)
	if err != nil {
		response.ServerError(c, err.Error())
		return
	}
	response.SuccessWithPage(c, total, page, pageSize, list)
}

// AdminSetStatus PUT /api/admin/comments/:id/status —— body: {status}
func (h *CommentHandler) AdminSetStatus(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "无效的评论ID")
		return
	}
	var req struct {
		Status int8 `json:"status"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}
	if err := h.svc.SetStatus(id, req.Status); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, nil)
}

// AdminReportList GET /api/admin/comment-reports —— status 默认 0(待处理),-1 全部
func (h *CommentHandler) AdminReportList(c *gin.Context) {
	status, _ := strconv.Atoi(c.DefaultQuery("status", "0"))
	page, pageSize := parsePage(c)
	list, total, err := h.svc.ReportList(status, page, pageSize)
	if err != nil {
		response.ServerError(c, err.Error())
		return
	}
	response.SuccessWithPage(c, total, page, pageSize, list)
}

// AdminHandleReport PUT /api/admin/comment-reports/:id —— body: {action: delete|dismiss, admin_note?}
func (h *CommentHandler) AdminHandleReport(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "无效的举报ID")
		return
	}
	var req struct {
		Action    string `json:"action"`
		AdminNote string `json:"admin_note"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}
	if err := h.svc.HandleReport(id, req.Action, req.AdminNote); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, nil)
}
