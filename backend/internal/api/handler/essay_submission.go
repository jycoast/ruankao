package handler

import (
	"software-exam/internal/application/service"
	"software-exam/pkg/response"
	"strconv"

	"github.com/gin-gonic/gin"
)

type EssaySubmissionHandler struct {
	svc *service.EssaySubmissionService
}

func NewEssaySubmissionHandler(svc *service.EssaySubmissionService) *EssaySubmissionHandler {
	return &EssaySubmissionHandler{svc: svc}
}

// Submit 用户投稿 POST /api/essays/submit
func (h *EssaySubmissionHandler) Submit(c *gin.Context) {
	userID := c.GetUint64("user_id")
	var req struct {
		Title    string  `json:"title" binding:"required"`
		Content  string  `json:"content" binding:"required"`
		Category string  `json:"category"`
		Year     int16   `json:"year"`
		Score    float32 `json:"score"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请填写标题和正文")
		return
	}
	if len([]rune(req.Content)) < 50 {
		response.BadRequest(c, "论文正文不得少于50字")
		return
	}
	id, err := h.svc.Submit(userID, req.Title, req.Content, req.Category, req.Year, req.Score)
	if err != nil {
		response.ServerError(c, err.Error())
		return
	}
	response.Success(c, gin.H{"id": id})
}

// MySubmissions 我的投稿列表 GET /api/essays/my-submissions
func (h *EssaySubmissionHandler) MySubmissions(c *gin.Context) {
	userID := c.GetUint64("user_id")
	page, pageSize := parsePage(c)
	list, total, err := h.svc.MySubmissions(userID, page, pageSize)
	if err != nil {
		response.ServerError(c, err.Error())
		return
	}
	response.SuccessWithPage(c, total, page, pageSize, list)
}

// AdminList 管理端投稿列表 GET /api/admin/essay-submissions
func (h *EssaySubmissionHandler) AdminList(c *gin.Context) {
	page, pageSize := parsePage(c)
	status, _ := strconv.Atoi(c.DefaultQuery("status", "-1"))
	list, total, err := h.svc.ListAll(page, pageSize, status)
	if err != nil {
		response.ServerError(c, err.Error())
		return
	}
	response.SuccessWithPage(c, total, page, pageSize, list)
}

// AdminGetOne 管理端查看单篇投稿（含正文）GET /api/admin/essay-submissions/:id
func (h *EssaySubmissionHandler) AdminGetOne(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "无效的ID")
		return
	}
	sub, err := h.svc.GetByID(id)
	if err != nil {
		response.NotFound(c, "投稿不存在")
		return
	}
	response.Success(c, sub)
}

// AdminReview 审核 PUT /api/admin/essay-submissions/:id/review
func (h *EssaySubmissionHandler) AdminReview(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "无效的ID")
		return
	}
	var req struct {
		Action      string  `json:"action" binding:"required"` // "approve" | "reject"
		Note        string  `json:"note"`
		ArticleType int8    `json:"article_type"` // 仅 approve 时有效
		Score       float32 `json:"score"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}
	switch req.Action {
	case "approve":
		if err := h.svc.Approve(id, req.Note, req.ArticleType); err != nil {
			response.ServerError(c, err.Error())
			return
		}
	case "reject":
		if err := h.svc.Reject(id, req.Note); err != nil {
			response.ServerError(c, err.Error())
			return
		}
	default:
		response.BadRequest(c, "action 必须为 approve 或 reject")
		return
	}
	response.Success(c, nil)
}
