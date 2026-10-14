package handler

import (
	"strconv"

	"software-exam/internal/application/service"
	"software-exam/internal/domain/repository"
	"software-exam/pkg/response"

	"github.com/gin-gonic/gin"
)

type FeedbackHandler struct {
	svc *service.FeedbackService
}

func NewFeedbackHandler(svc *service.FeedbackService) *FeedbackHandler {
	return &FeedbackHandler{svc: svc}
}

// Submit 用户提交网站反馈/建议 POST /api/feedbacks
func (h *FeedbackHandler) Submit(c *gin.Context) {
	userID := c.GetUint64("user_id")

	var req struct {
		FeedbackType int8   `json:"feedback_type" binding:"required,min=1,max=4"`
		QuestionID   uint64 `json:"question_id"`
		// Content 只必填、不设 min=5：反馈已是富文本 HTML，串长度算不出"几个字"
		//（<p>ab</p> 有 8 字符却只有 2 个可见字）。「至少 5 个字」改由
		// FeedbackService.Submit 按剥离标签后的可见文字校验（sanitizeFeedbackContent）。
		Content string `json:"content" binding:"required"`
		Contact string `json:"contact"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请填写反馈内容")
		return
	}

	if err := h.svc.Submit(userID, req.FeedbackType, req.QuestionID, req.Content, req.Contact); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, nil)
}

// Mine 用户查看自己提交的反馈及处理结果 GET /api/feedbacks/mine
func (h *FeedbackHandler) Mine(c *gin.Context) {
	userID := c.GetUint64("user_id")
	page, pageSize := parsePage(c)

	list, total, err := h.svc.MyFeedbacks(userID, page, pageSize)
	if err != nil {
		response.ServerError(c, err.Error())
		return
	}
	response.SuccessWithPage(c, total, page, pageSize, list)
}

// ListAll 管理端：获取所有反馈 GET /api/admin/feedbacks
// 支持 status（-1全部/0待处理/1已处理/2已忽略）、type（-1全部/1功能建议/2问题反馈/3内容纠错/4其他）、keyword 过滤。
func (h *FeedbackHandler) ListAll(c *gin.Context) {
	page, pageSize := parsePage(c)
	status, _ := strconv.Atoi(c.DefaultQuery("status", "-1"))
	fbType, _ := strconv.ParseInt(c.DefaultQuery("type", "-1"), 10, 8)
	keyword := c.Query("keyword")

	list, total, err := h.svc.ListAll(repository.FeedbackListReq{
		Page:     page,
		PageSize: pageSize,
		Status:   status,
		Type:     int8(fbType),
		Keyword:  keyword,
	})
	if err != nil {
		response.ServerError(c, err.Error())
		return
	}
	response.SuccessWithPage(c, total, page, pageSize, list)
}

// UpdateStatus 管理端：更新反馈状态 PUT /api/admin/feedbacks/:id
func (h *FeedbackHandler) UpdateStatus(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "无效的ID")
		return
	}
	var req struct {
		Status    int8   `json:"status"`
		AdminNote string `json:"admin_note"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}
	if req.Status != 1 && req.Status != 2 {
		response.BadRequest(c, "无效的状态")
		return
	}
	if err := h.svc.UpdateStatus(id, req.Status, req.AdminNote); err != nil {
		response.ServerError(c, err.Error())
		return
	}
	response.Success(c, nil)
}

// CountPending 管理端：待处理数量 GET /api/admin/feedbacks/pending-count
func (h *FeedbackHandler) CountPending(c *gin.Context) {
	count, err := h.svc.CountPending()
	if err != nil {
		response.ServerError(c, err.Error())
		return
	}
	response.Success(c, gin.H{"count": count})
}
