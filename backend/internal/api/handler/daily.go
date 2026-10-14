package handler

import (
	"software-exam/internal/application/service"
	"software-exam/pkg/response"
	"strconv"

	"github.com/gin-gonic/gin"
)

type DailyHandler struct {
	svc *service.DailyService
}

func NewDailyHandler(svc *service.DailyService) *DailyHandler {
	return &DailyHandler{svc: svc}
}

// Today GET /api/daily/today —— 今日一练题目 + 签到状态 + 当日进度
func (h *DailyHandler) Today(c *gin.Context) {
	userID := c.GetUint64("user_id")
	subjectID, _ := strconv.ParseUint(c.DefaultQuery("subject_id", "0"), 10, 64)
	res, err := h.svc.GetToday(userID, subjectID)
	if err != nil {
		response.ServerError(c, err.Error())
		return
	}
	response.Success(c, res)
}

// Checkin POST /api/daily/checkin —— 完成今日一练后签到
func (h *DailyHandler) Checkin(c *gin.Context) {
	var req struct {
		SubjectID     uint64 `json:"subject_id"`
		QuestionCount int    `json:"question_count" binding:"required,min=1"`
		CorrectCount  int    `json:"correct_count"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请先完成今日一练")
		return
	}
	userID := c.GetUint64("user_id")
	if err := h.svc.Checkin(userID, req.SubjectID, req.QuestionCount, req.CorrectCount); err != nil {
		response.ServerError(c, err.Error())
		return
	}
	response.Success(c, nil)
}

// SaveSession POST /api/daily/session —— 保存每日一练进度（防抖写入，best-effort）
func (h *DailyHandler) SaveSession(c *gin.Context) {
	userID := c.GetUint64("user_id")
	var req struct {
		SubjectID    uint64              `json:"subject_id"`
		CurrentIndex int                 `json:"current_index"`
		CurrentBlank int                 `json:"current_blank"`
		Answers      map[string]string   `json:"answers"`
		ShowResult   bool                `json:"show_result"`
		Result       interface{}         `json:"result"`
		CorrectCount int                 `json:"correct_count"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}
	if err := h.svc.SaveSession(userID, req.SubjectID, &service.DailySessionData{
		CurrentIndex: req.CurrentIndex,
		CurrentBlank: req.CurrentBlank,
		Answers:      req.Answers,
		ShowResult:   req.ShowResult,
		Result:       req.Result,
		CorrectCount: req.CorrectCount,
	}); err != nil {
		response.ServerError(c, err.Error())
		return
	}
	response.Success(c, nil)
}