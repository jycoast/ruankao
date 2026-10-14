package handler

import (
	"strconv"
	"time"

	"software-exam/internal/application/service"
	"software-exam/pkg/response"

	"github.com/gin-gonic/gin"
)

type ExamScheduleHandler struct {
	svc *service.ExamScheduleService
}

func NewExamScheduleHandler(svc *service.ExamScheduleService) *ExamScheduleHandler {
	return &ExamScheduleHandler{svc: svc}
}

// List 公开只读：按考试日期升序返回全部排期
func (h *ExamScheduleHandler) List(c *gin.Context) {
	list, err := h.svc.List()
	if err != nil {
		response.ServerError(c, err.Error())
		return
	}
	response.Success(c, list)
}

func (h *ExamScheduleHandler) Create(c *gin.Context) {
	var req struct {
		Label    string `json:"label" binding:"required"`
		ExamDate string `json:"exam_date" binding:"required,datetime=2006-01-02"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误：需要 label 和 YYYY-MM-DD 的 exam_date")
		return
	}
	examDate, err := time.ParseInLocation("2006-01-02", req.ExamDate, time.Local)
	if err != nil {
		response.BadRequest(c, "考试日期格式应为 YYYY-MM-DD")
		return
	}
	id, err := h.svc.Create(req.Label, examDate)
	if err != nil {
		response.ServerError(c, err.Error())
		return
	}
	response.Success(c, gin.H{"id": id})
}

func (h *ExamScheduleHandler) Update(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "无效的ID")
		return
	}
	var req struct {
		Label    string `json:"label" binding:"required"`
		ExamDate string `json:"exam_date" binding:"required,datetime=2006-01-02"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误：需要 label 和 YYYY-MM-DD 的 exam_date")
		return
	}
	examDate, err := time.ParseInLocation("2006-01-02", req.ExamDate, time.Local)
	if err != nil {
		response.BadRequest(c, "考试日期格式应为 YYYY-MM-DD")
		return
	}
	if err := h.svc.Update(id, req.Label, examDate); err != nil {
		if err.Error() == "record not found" {
			response.NotFound(c, "排期不存在")
			return
		}
		response.ServerError(c, err.Error())
		return
	}
	response.Success(c, nil)
}

func (h *ExamScheduleHandler) Delete(c *gin.Context) {
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
