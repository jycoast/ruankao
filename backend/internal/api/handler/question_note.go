package handler

import (
	"errors"

	"software-exam/internal/application/service"
	"software-exam/pkg/response"
	"strconv"

	"github.com/gin-gonic/gin"
)

type QuestionNoteHandler struct {
	svc *service.QuestionNoteService
}

func NewQuestionNoteHandler(svc *service.QuestionNoteService) *QuestionNoteHandler {
	return &QuestionNoteHandler{svc: svc}
}

// Get GET /api/questions/:id/note
func (h *QuestionNoteHandler) Get(c *gin.Context) {
	userID := c.GetUint64("user_id")
	questionID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "无效的题目ID")
		return
	}
	note, err := h.svc.Get(userID, questionID)
	if err != nil {
		// no note yet — return empty
		response.Success(c, gin.H{"content": "", "exists": false})
		return
	}
	response.Success(c, gin.H{"content": note.Content, "exists": true, "updated_at": note.UpdatedAt})
}

// Save PUT /api/questions/:id/note
func (h *QuestionNoteHandler) Save(c *gin.Context) {
	userID := c.GetUint64("user_id")
	questionID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "无效的题目ID")
		return
	}
	var req struct {
		Content string `json:"content" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "内容不能为空")
		return
	}
	if err := h.svc.Save(userID, questionID, req.Content); err != nil {
		response.ServerError(c, err.Error())
		return
	}
	response.Success(c, nil)
}

// MyNotes GET /api/my/notes —— 当前用户的分页笔记（带题目概要），支持 keyword 搜索
func (h *QuestionNoteHandler) MyNotes(c *gin.Context) {
	userID := c.GetUint64("user_id")
	page, pageSize := parsePage(c)
	list, total, err := h.svc.ListByUser(userID, page, pageSize, c.Query("keyword"))
	if err != nil {
		response.ServerError(c, err.Error())
		return
	}
	response.SuccessWithPage(c, total, page, pageSize, list)
}

// Detail GET /api/my/notes/:question_id —— 单条笔记详情：笔记 + 完整题目（含选项）。
//
// 两条容易踩的约束，改这个函数前先读：
//  1. 只对**已写过笔记**的题目返回题面。服务层查不到笔记即 ErrNoteNotFound，这里回 404。
//     这个 404 是安全边界，不要改成返回空对象 + 200。
//  2. 本路由**故意不挂** mw.TrackQA：用户复习自己的笔记不该被计入反爬的
//     「24h 内不同题目数」，那条规则会把认真复习的人误判成批量爬取。
//     另外参数名是 :question_id，而 TrackQA 从 c.Param("id") 取题号
//     （api/middleware/antiscrape.go 的 TrackQuestionAccess 里那句
//     strconv.ParseUint(c.Param("id"), 10, 64)），取到空串后静默 return ——
//     将来想补挂中间件，直接挂是**空转**的，得先处理参数名。
func (h *QuestionNoteHandler) Detail(c *gin.Context) {
	userID := c.GetUint64("user_id")
	questionID, err := strconv.ParseUint(c.Param("question_id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "无效的题目ID")
		return
	}
	note, question, err := h.svc.Detail(userID, questionID)
	if errors.Is(err, service.ErrNoteNotFound) {
		response.NotFound(c, "笔记不存在")
		return
	}
	if err != nil {
		response.ServerError(c, err.Error())
		return
	}
	response.Success(c, gin.H{"note": note, "question": question})
}

// Delete DELETE /api/questions/:id/note
func (h *QuestionNoteHandler) Delete(c *gin.Context) {
	userID := c.GetUint64("user_id")
	questionID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "无效的题目ID")
		return
	}
	if err := h.svc.Delete(userID, questionID); err != nil {
		response.ServerError(c, err.Error())
		return
	}
	response.Success(c, nil)
}
