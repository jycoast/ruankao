package handler

import (
	"software-exam/internal/application/service"
	"software-exam/internal/domain/repository"
	"software-exam/pkg/response"
	"strconv"

	"github.com/gin-gonic/gin"
)

type QuestionHandler struct {
	questionService *service.QuestionService
}

func NewQuestionHandler(questionService *service.QuestionService) *QuestionHandler {
	return &QuestionHandler{questionService: questionService}
}

func (h *QuestionHandler) List(c *gin.Context) {
	page, pageSize := parsePage(c)
	qType, _ := strconv.Atoi(c.DefaultQuery("question_type", "0"))
	categoryID, _ := strconv.ParseUint(c.DefaultQuery("category_id", "0"), 10, 64)
	year, _ := strconv.Atoi(c.DefaultQuery("year", "0"))
	difficulty, _ := strconv.Atoi(c.DefaultQuery("difficulty", "0"))
	keyword := c.Query("keyword")
	subjectID, _ := strconv.ParseUint(c.DefaultQuery("subject_id", "0"), 10, 64)

	req := repository.QuestionListReq{
		Page:         page,
		PageSize:     pageSize,
		QuestionType: int8(qType),
		CategoryID:   categoryID,
		Year:         int16(year),
		Difficulty:   int8(difficulty),
		Keyword:      keyword,
		SubjectID:    subjectID,
	}

	questions, total, err := h.questionService.List(req)
	if err != nil {
		response.ServerError(c, err.Error())
		return
	}
	response.SuccessWithPage(c, total, page, pageSize, questions)
}

func (h *QuestionHandler) GetByID(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "无效的ID")
		return
	}
	question, err := h.questionService.GetByID(id)
	if err != nil {
		response.NotFound(c, "题目不存在")
		return
	}
	response.Success(c, question)
}

func (h *QuestionHandler) Random(c *gin.Context) {
	qType, _ := strconv.Atoi(c.DefaultQuery("question_type", "1"))
	count, _ := strconv.Atoi(c.DefaultQuery("count", "10"))
	count = clampCount(count, 10, maxRandomCount)
	knowledgePoint := c.Query("knowledge_point")
	subjectID, _ := strconv.ParseUint(c.DefaultQuery("subject_id", "0"), 10, 64)
	questions, err := h.questionService.GetRandom(int8(qType), count, knowledgePoint, subjectID)
	if err != nil {
		response.ServerError(c, err.Error())
		return
	}
	response.Success(c, questions)
}

func (h *QuestionHandler) Categories(c *gin.Context) {
	subjectID, _ := strconv.ParseUint(c.DefaultQuery("subject_id", "0"), 10, 64)
	categories, err := h.questionService.GetCategories(subjectID)
	if err != nil {
		response.ServerError(c, err.Error())
		return
	}
	response.Success(c, categories)
}

func (h *QuestionHandler) Create(c *gin.Context) {
	var question service.QuestionRequest
	if err := c.ShouldBindJSON(&question); err != nil {
		response.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	id, err := h.questionService.Create(&question)
	if err != nil {
		response.ServerError(c, err.Error())
		return
	}
	response.Success(c, gin.H{"id": id})
}

func (h *QuestionHandler) Update(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "无效的ID")
		return
	}
	var question service.QuestionRequest
	if err := c.ShouldBindJSON(&question); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}
	if err := h.questionService.Update(id, &question); err != nil {
		response.ServerError(c, err.Error())
		return
	}
	response.Success(c, nil)
}

func (h *QuestionHandler) Delete(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "无效的ID")
		return
	}
	if err := h.questionService.Delete(id); err != nil {
		response.ServerError(c, err.Error())
		return
	}
	response.Success(c, nil)
}

func (h *QuestionHandler) Sort(c *gin.Context) {
	var req struct {
		IDs []uint64 `json:"ids"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || len(req.IDs) == 0 {
		response.BadRequest(c, "参数错误：ids 顺序不能为空")
		return
	}
	if err := h.questionService.UpdateSort(req.IDs); err != nil {
		response.ServerError(c, err.Error())
		return
	}
	response.Success(c, nil)
}
