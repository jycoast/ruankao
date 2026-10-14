package handler

import (
	"software-exam/internal/application/service"
	"software-exam/pkg/response"
	"strconv"

	"github.com/gin-gonic/gin"
)

type PracticeHandler struct {
	practiceService *service.PracticeService
}

func NewPracticeHandler(practiceService *service.PracticeService) *PracticeHandler {
	return &PracticeHandler{practiceService: practiceService}
}

type SubmitAnswerRequest struct {
	QuestionID   uint64 `json:"question_id" binding:"required"`
	UserAnswer   string `json:"user_answer" binding:"required"`
	PracticeType int8   `json:"practice_type" binding:"required"`
	TimeSpent    int    `json:"time_spent"`
	// 以下三项定位分类刷题的草稿，用于「继续刷题」还原答题卡。
	// 其他答题入口（错题复习、试卷）不传，此时不落草稿。
	SubjectID      uint64 `json:"subject_id"`
	QuestionType   int8   `json:"question_type"`
	KnowledgePoint string `json:"knowledge_point"`
}

func (h *PracticeHandler) SubmitAnswer(c *gin.Context) {
	userID := c.GetUint64("user_id")
	var req SubmitAnswerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}
	result, err := h.practiceService.SubmitAnswer(userID, req.QuestionID, req.UserAnswer, req.PracticeType, req.TimeSpent)
	if err != nil {
		response.ServerError(c, err.Error())
		return
	}
	// 草稿写失败不影响本次作答已入库的事实，不因此回报错误
	if req.KnowledgePoint != "" {
		h.practiceService.SaveSessionAnswer(userID, req.SubjectID, req.QuestionType, req.KnowledgePoint, req.QuestionID, req.UserAnswer)
	}
	response.Success(c, result)
}

type StartPracticeRequest struct {
	SubjectID      uint64 `json:"subject_id"`
	QuestionType   int8   `json:"question_type" binding:"required"`
	KnowledgePoint string `json:"knowledge_point" binding:"required"`
	Fresh          bool   `json:"fresh"` // true=从头开始（丢弃上次答题记录），false=继续刷题
}

// StartPractice POST /api/practice/start —— 开一轮分类刷题。
func (h *PracticeHandler) StartPractice(c *gin.Context) {
	userID := c.GetUint64("user_id")
	var req StartPracticeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}
	data, err := h.practiceService.StartPractice(userID, req.SubjectID, req.QuestionType, req.KnowledgePoint, req.Fresh)
	if err != nil {
		response.ServerError(c, err.Error())
		return
	}
	response.Success(c, data)
}

// GetSessionState GET /api/practice/session —— 某知识点是否有可继续的进度。
func (h *PracticeHandler) GetSessionState(c *gin.Context) {
	userID := c.GetUint64("user_id")
	subjectID, _ := strconv.ParseUint(c.DefaultQuery("subject_id", "0"), 10, 64)
	qType, _ := strconv.Atoi(c.DefaultQuery("question_type", "0"))
	kp := c.Query("knowledge_point")
	response.Success(c, gin.H{
		"resumable": kp != "" && h.practiceService.HasSession(userID, subjectID, int8(qType), kp),
	})
}

func (h *PracticeHandler) GetWrongQuestions(c *gin.Context) {
	userID := c.GetUint64("user_id")
	status, _ := strconv.Atoi(c.DefaultQuery("status", "0"))
	page, pageSize := parsePage(c)
	wqs, total, err := h.practiceService.GetWrongQuestions(userID, int8(status), page, pageSize)
	if err != nil {
		response.ServerError(c, err.Error())
		return
	}
	response.SuccessWithPage(c, total, page, pageSize, wqs)
}

func (h *PracticeHandler) DeleteWrongQuestion(c *gin.Context) {
	userID := c.GetUint64("user_id")
	questionID, _ := strconv.ParseUint(c.Param("question_id"), 10, 64)
	if err := h.practiceService.DeleteWrongQuestion(userID, questionID); err != nil {
		response.ServerError(c, err.Error())
		return
	}
	response.Success(c, nil)
}

func (h *PracticeHandler) GetDueReview(c *gin.Context) {
	userID := c.GetUint64("user_id")
	page, pageSize := parsePage(c)
	wqs, total, err := h.practiceService.GetDueReview(userID, page, pageSize)
	if err != nil {
		response.ServerError(c, err.Error())
		return
	}
	response.SuccessWithPage(c, total, page, pageSize, wqs)
}

// GetReport GET /api/user/report —— 学习数据报告
func (h *PracticeHandler) GetReport(c *gin.Context) {
	userID := c.GetUint64("user_id")
	subjectID, _ := strconv.ParseUint(c.DefaultQuery("subject_id", "0"), 10, 64)
	report, err := h.practiceService.GetReport(userID, subjectID)
	if err != nil {
		response.ServerError(c, err.Error())
		return
	}
	response.Success(c, report)
}

// GetDiagnosisSummary GET /api/diagnosis/summary —— AI 诊断报告数据（会员专属）。
// 与 GetReport 同构：读 user_id + query subject_id，返回薄弱点画像聚合。
func (h *PracticeHandler) GetDiagnosisSummary(c *gin.Context) {
	userID := c.GetUint64("user_id")
	subjectID, _ := strconv.ParseUint(c.DefaultQuery("subject_id", "0"), 10, 64)
	sum, err := h.practiceService.GetDiagnosisSummary(userID, subjectID)
	if err != nil {
		response.ServerError(c, err.Error())
		return
	}
	response.Success(c, sum)
}

type ReviewRequest struct {
	QuestionID uint64 `json:"question_id" binding:"required"`
	IsCorrect  *bool  `json:"is_correct" binding:"required"`
}

func (h *PracticeHandler) Review(c *gin.Context) {
	userID := c.GetUint64("user_id")
	var req ReviewRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}
	if err := h.practiceService.Review(userID, req.QuestionID, *req.IsCorrect); err != nil {
		response.ServerError(c, err.Error())
		return
	}
	response.Success(c, nil)
}

func (h *PracticeHandler) GetHistory(c *gin.Context) {
	userID := c.GetUint64("user_id")
	page, pageSize := parsePage(c)
	records, total, err := h.practiceService.GetHistory(userID, page, pageSize)
	if err != nil {
		response.ServerError(c, err.Error())
		return
	}
	response.SuccessWithPage(c, total, page, pageSize, records)
}

func (h *PracticeHandler) GetCategoryProgress(c *gin.Context) {
	userID := c.GetUint64("user_id")
	data, err := h.practiceService.GetCategoryProgress(userID)
	if err != nil {
		response.ServerError(c, err.Error())
		return
	}
	response.Success(c, data)
}
