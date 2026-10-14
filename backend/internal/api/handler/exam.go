package handler

import (
	"strconv"
	"time"

	"software-exam/internal/application/service"
	"software-exam/internal/domain/repository"
	"software-exam/pkg/response"

	"github.com/gin-gonic/gin"
)

type ExamHandler struct {
	examService       *service.ExamService
	membershipService *service.MembershipService
}

func NewExamHandler(examService *service.ExamService, membershipService *service.MembershipService) *ExamHandler {
	return &ExamHandler{examService: examService, membershipService: membershipService}
}

func (h *ExamHandler) ListPapers(c *gin.Context) {
	paperType, _ := strconv.Atoi(c.DefaultQuery("paper_type", "0"))
	page, pageSize := parsePage(c)
	subjectID, _ := strconv.ParseUint(c.DefaultQuery("subject_id", "0"), 10, 64)
	papers, total, err := h.examService.ListPapers(int8(paperType), page, pageSize, subjectID)
	if err != nil {
		response.ServerError(c, err.Error())
		return
	}
	response.SuccessWithPage(c, total, page, pageSize, papers)
}

func (h *ExamHandler) GetPaper(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "无效的ID")
		return
	}
	paper, err := h.examService.GetPaperWithQuestions(id)
	if err != nil {
		response.NotFound(c, "试卷不存在")
		return
	}
	// 近三年真题练习/考试为会员专属：非会员仍返回卷信息，但不返回题目，防直接调 API 绕过。
	// 只拦真实真题（Source==0）；生成模拟卷（Source==1）即使年份在近三年也保持免费。
	if paper.Source == 0 && paper.Year >= int16(time.Now().Year()-2) && !h.membershipService.IsActive(c.GetUint64("user_id")) {
		paper.Questions = nil
	}
	response.Success(c, paper)
}

func (h *ExamHandler) StartExam(c *gin.Context) {
	userID := c.GetUint64("user_id")
	paperID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "无效的ID")
		return
	}
	record, err := h.examService.StartExam(userID, paperID)
	if err != nil {
		response.ServerError(c, err.Error())
		return
	}
	response.Success(c, record)
}

type SubmitExamRequest struct {
	ExamRecordID uint64             `json:"exam_record_id" binding:"required"`
	Answers      []service.ExamAnswer `json:"answers" binding:"required"`
}

func (h *ExamHandler) SubmitExam(c *gin.Context) {
	userID := c.GetUint64("user_id")
	var req SubmitExamRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}
	result, err := h.examService.SubmitExam(userID, req.ExamRecordID, req.Answers)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, result)
}

func (h *ExamHandler) GetExamRecords(c *gin.Context) {
	userID := c.GetUint64("user_id")
	page, pageSize := parsePage(c)
	records, total, err := h.examService.GetExamRecords(userID, page, pageSize)
	if err != nil {
		response.ServerError(c, err.Error())
		return
	}
	response.SuccessWithPage(c, total, page, pageSize, records)
}

type GenerateMockRequest struct {
	SubjectID uint64 `json:"subject_id"`
	Years     int    `json:"years"`   // 近几年，<=0 表示不限
	Single    int    `json:"single"`  // 单选题数量
	Multiple  int    `json:"multiple"` // 多选题数量
	Case      int    `json:"case"`    // 案例题数量
	Essay     int    `json:"essay"`   // 论文题数量
}

// clampTypeCount 将单题型数量限制在 0..maxPer（允许为 0，表示该题型不出题）
func clampTypeCount(n, maxPer int) int {
	if n < 0 {
		return 0
	}
	if n > maxPer {
		return maxPer
	}
	return n
}

func (h *ExamHandler) GenerateMock(c *gin.Context) {
	userID := c.GetUint64("user_id")
	var req GenerateMockRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}

	// 单选/多选上限 50，案例上限 10，论文上限 5；总量上限 100
	spec := service.MockSpec{
		1: clampTypeCount(req.Single, 50),
		2: clampTypeCount(req.Multiple, 50),
		3: clampTypeCount(req.Case, 10),
		4: clampTypeCount(req.Essay, 5),
	}
	total := spec[1] + spec[2] + spec[3] + spec[4]
	if total == 0 {
		response.BadRequest(c, "请至少选择一种题型的数量")
		return
	}
	if total > 100 {
		response.BadRequest(c, "单份模拟卷题量不能超过 100 题")
		return
	}
	if req.Years < 0 {
		req.Years = 0
	}

	paperID, err := h.examService.GenerateMockPaper(userID, req.SubjectID, spec, req.Years)
	if err != nil {
		response.ServerError(c, err.Error())
		return
	}
	response.Success(c, gin.H{"paper_id": paperID})
}

// ---- 管理端：试卷管理（路由挂在 /api/admin/papers，由 AdminMiddleware 保护） ----

func (h *ExamHandler) AdminListPapers(c *gin.Context) {
	page, pageSize := parsePage(c)
	paperType, _ := strconv.Atoi(c.DefaultQuery("paper_type", "0"))
	year, _ := strconv.Atoi(c.DefaultQuery("year", "0"))
	subjectID, _ := strconv.ParseUint(c.DefaultQuery("subject_id", "0"), 10, 64)
	req := repository.AdminPaperListReq{
		Page:      page,
		PageSize:  pageSize,
		PaperType: int8(paperType),
		Year:      int16(year),
		Keyword:   c.Query("keyword"),
		SubjectID: subjectID,
	}
	papers, total, err := h.examService.ListAdminPapers(req)
	if err != nil {
		response.ServerError(c, err.Error())
		return
	}
	response.SuccessWithPage(c, total, page, pageSize, papers)
}

func (h *ExamHandler) AdminCreatePaper(c *gin.Context) {
	var req service.PaperRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	adminID := c.GetUint64("user_id")
	id, err := h.examService.CreatePaper(adminID, &req)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, gin.H{"id": id})
}

func (h *ExamHandler) AdminUpdatePaper(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "无效的ID")
		return
	}
	var req service.PaperRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	if err := h.examService.UpdatePaper(id, &req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, nil)
}

func (h *ExamHandler) AdminDeletePaper(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "无效的ID")
		return
	}
	if err := h.examService.DeletePaper(id); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, nil)
}

func (h *ExamHandler) AdminGetPaperQuestions(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "无效的ID")
		return
	}
	questions, err := h.examService.GetPaperQuestions(id)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, questions)
}

type ReplacePaperQuestionsRequest struct {
	QuestionIDs []uint64 `json:"question_ids"`
}

func (h *ExamHandler) AdminReplacePaperQuestions(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "无效的ID")
		return
	}
	var req ReplacePaperQuestionsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	if err := h.examService.ReplacePaperQuestions(id, req.QuestionIDs); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, nil)
}

// ---- 真题练习模式断点草稿 ----

type SavePracticeSessionRequest struct {
	PaperID         uint64                            `json:"paper_id" binding:"required"`
	Mode            string                            `json:"mode"`
	Answers         map[string]map[string]string      `json:"answers"`
	PracticeResults map[string]service.PaperSessionResult `json:"practiceResults"`
	CurrentIndex    int                               `json:"currentIndex"`
}

// GetPracticeSession GET /api/exam/practice-session —— 某卷练习模式是否有可继续的进度。
func (h *ExamHandler) GetPracticeSession(c *gin.Context) {
	userID := c.GetUint64("user_id")
	paperID, _ := strconv.ParseUint(c.DefaultQuery("paper_id", "0"), 10, 64)
	mode := c.DefaultQuery("mode", "practice")
	data, ok := h.examService.GetPracticeSessionState(userID, paperID, mode)
	if !ok {
		response.Success(c, gin.H{"resumable": false, "session": nil})
		return
	}
	response.Success(c, gin.H{"resumable": true, "session": data})
}

// SavePracticeSession POST /api/exam/practice-session —— 落练习草稿（防抖后提交，best-effort）。
func (h *ExamHandler) SavePracticeSession(c *gin.Context) {
	userID := c.GetUint64("user_id")
	var req SavePracticeSessionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}
	if err := h.examService.SavePracticeSession(userID, req.PaperID, req.Mode, &service.PaperSessionData{
		Answers:         req.Answers,
		PracticeResults: req.PracticeResults,
		CurrentIndex:    req.CurrentIndex,
	}); err != nil {
		response.ServerError(c, err.Error())
		return
	}
	response.Success(c, nil)
}

// ClearPracticeSession DELETE /api/exam/practice-session —— 清练习草稿（「从头开始」/完成时）。
func (h *ExamHandler) ClearPracticeSession(c *gin.Context) {
	userID := c.GetUint64("user_id")
	paperID, _ := strconv.ParseUint(c.DefaultQuery("paper_id", "0"), 10, 64)
	mode := c.DefaultQuery("mode", "practice")
	if err := h.examService.ClearPracticeSession(userID, paperID, mode); err != nil {
		response.ServerError(c, err.Error())
		return
	}
	response.Success(c, nil)
}
