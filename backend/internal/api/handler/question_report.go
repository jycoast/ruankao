package handler

import (
	"software-exam/internal/application/service"
	"software-exam/pkg/response"
	"strconv"

	"github.com/gin-gonic/gin"
)

type QuestionReportHandler struct {
	svc *service.QuestionReportService
}

func NewQuestionReportHandler(svc *service.QuestionReportService) *QuestionReportHandler {
	return &QuestionReportHandler{svc: svc}
}

// Submit 用户提交题目问题报告 POST /api/questions/:id/report
func (h *QuestionReportHandler) Submit(c *gin.Context) {
	questionID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "无效的题目ID")
		return
	}
	userID := c.GetUint64("user_id")

	var req struct {
		ReportType  int8   `json:"report_type" binding:"required,min=1,max=5"`
		Description string `json:"description" binding:"required,min=5"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请填写问题描述（至少5个字）")
		return
	}

	if err := h.svc.Submit(questionID, userID, req.ReportType, req.Description); err != nil {
		response.ServerError(c, err.Error())
		return
	}
	response.Success(c, nil)
}

// Mine 用户查看自己提交的纠错及处理结果 GET /api/questions/my-reports
func (h *QuestionReportHandler) Mine(c *gin.Context) {
	userID := c.GetUint64("user_id")
	page, pageSize := parsePage(c)

	list, total, err := h.svc.MyReports(userID, page, pageSize)
	if err != nil {
		response.ServerError(c, err.Error())
		return
	}
	response.SuccessWithPage(c, total, page, pageSize, list)
}

// ListAll 管理端：获取所有报告 GET /api/admin/reports
func (h *QuestionReportHandler) ListAll(c *gin.Context) {
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

// UpdateStatus 管理端：更新报告状态 PUT /api/admin/reports/:id
func (h *QuestionReportHandler) UpdateStatus(c *gin.Context) {
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

// CountPending 管理端：待处理数量 GET /api/admin/reports/pending-count
func (h *QuestionReportHandler) CountPending(c *gin.Context) {
	count, err := h.svc.CountPending()
	if err != nil {
		response.ServerError(c, err.Error())
		return
	}
	response.Success(c, gin.H{"count": count})
}
