package handler

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"software-exam/internal/application/service"
	"software-exam/pkg/auth"
	"software-exam/pkg/response"

	"github.com/gin-gonic/gin"
)

type AIHandler struct {
	aiService         *service.AIService
	membershipService *service.MembershipService
}

func NewAIHandler(aiService *service.AIService, membershipService *service.MembershipService) *AIHandler {
	return &AIHandler{aiService: aiService, membershipService: membershipService}
}

// ---- 状态 ----

// GET /api/ai/status
// 会员/管理员/自带 key 的用户返回 trial_limit=0，表示不受免费额度限制。
func (h *AIHandler) Status(c *gin.Context) {
	userID := c.GetUint64("user_id")
	role, _ := c.Get("role")

	// 虚拟游客(role 2):AI 全站对游客关闭,返回不可用态,不展示试用配额。
	if r, ok := role.(int8); ok && r == auth.RoleGuest {
		response.Success(c, gin.H{
			"enabled":     false,
			"trial_used":  0,
			"trial_limit": 0,
			"own_config":  false,
		})
		return
	}

	isAdmin := false
	if r, ok := role.(int8); ok && r == 1 {
		isAdmin = true
	}

	data := gin.H{
		"enabled":     h.aiService.Enabled(userID),
		"trial_used":  0,
		"trial_limit": 0,
		"own_config":  h.aiService.UsesOwnConfig(userID),
	}
	if !isAdmin && !h.membershipService.IsActive(userID) {
		used, limit := h.aiService.TrialQuota(userID)
		data["trial_used"] = used
		data["trial_limit"] = limit
	}
	response.Success(c, data)
}

// ---- 管理员配置 ----

// GET /api/admin/ai/config
func (h *AIHandler) GetConfig(c *gin.Context) {
	data, err := h.aiService.GetConfigMasked()
	if err != nil {
		response.ServerError(c, err.Error())
		return
	}
	response.Success(c, data)
}

type updateConfigRequest struct {
	BaseURL        string  `json:"base_url"`
	APIKey         string  `json:"api_key"` // 空则保留原值
	Model          string  `json:"model"`
	Temperature    float64 `json:"temperature"`
	MaxTokens      int     `json:"max_tokens"`
	Enabled        int8    `json:"enabled"`
	FreeTrialLimit int     `json:"free_trial_limit"`
}

// PUT /api/admin/ai/config
func (h *AIHandler) UpdateConfig(c *gin.Context) {
	var req updateConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}
	if err := h.aiService.UpdateConfig(req.BaseURL, req.APIKey, req.Model, req.Temperature, req.MaxTokens, req.Enabled, req.FreeTrialLimit); err != nil {
		response.ServerError(c, err.Error())
		return
	}
	response.Success(c, nil)
}

// POST /api/admin/ai/test
func (h *AIHandler) TestConfig(c *gin.Context) {
	reply, err := h.aiService.TestConfig()
	if err != nil {
		response.ServerError(c, err.Error())
		return
	}
	response.Success(c, gin.H{"reply": reply})
}

// ---- 用量统计（管理员） ----

// GET /api/admin/ai/usage?days=7
func (h *AIHandler) Usage(c *gin.Context) {
	days, _ := strconv.Atoi(c.DefaultQuery("days", "7"))
	stats, err := h.aiService.UsageStats(days)
	if err != nil {
		response.ServerError(c, err.Error())
		return
	}
	response.Success(c, stats)
}

// GET /api/admin/ai/logs?scene=&page=&page_size=
func (h *AIHandler) ListCallLogs(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	list, total, err := h.aiService.ListCallLogs(c.Query("scene"), page, pageSize)
	if err != nil {
		response.ServerError(c, err.Error())
		return
	}
	response.Success(c, gin.H{"items": list, "total": total})
}

// ---- 会员自定义配置 ----

// GET /api/ai/my-config
func (h *AIHandler) GetMyConfig(c *gin.Context) {
	userID := c.GetUint64("user_id")
	data, err := h.aiService.GetUserConfigMasked(userID)
	if err != nil {
		response.ServerError(c, err.Error())
		return
	}
	response.Success(c, data)
}

// PUT /api/ai/my-config
func (h *AIHandler) UpdateMyConfig(c *gin.Context) {
	userID := c.GetUint64("user_id")
	var req updateConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}
	if err := h.aiService.UpdateUserConfig(userID, req.BaseURL, req.APIKey, req.Model, req.Temperature, req.MaxTokens, req.Enabled); err != nil {
		response.ServerError(c, err.Error())
		return
	}
	response.Success(c, nil)
}

// POST /api/ai/my-config/test
func (h *AIHandler) TestMyConfig(c *gin.Context) {
	userID := c.GetUint64("user_id")
	reply, err := h.aiService.TestUserConfig(userID)
	if err != nil {
		response.ServerError(c, err.Error())
		return
	}
	response.Success(c, gin.H{"reply": reply})
}

// ---- 会话 ----

// GET /api/ai/conversations
func (h *AIHandler) ListConversations(c *gin.Context) {
	userID := c.GetUint64("user_id")
	list, err := h.aiService.ListConversations(userID)
	if err != nil {
		response.ServerError(c, err.Error())
		return
	}
	response.Success(c, list)
}

// POST /api/ai/conversations
func (h *AIHandler) CreateConversation(c *gin.Context) {
	userID := c.GetUint64("user_id")
	var req struct {
		Title string `json:"title"`
	}
	_ = c.ShouldBindJSON(&req)
	conv, err := h.aiService.CreateConversation(userID, req.Title)
	if err != nil {
		response.ServerError(c, err.Error())
		return
	}
	response.Success(c, conv)
}

// GET /api/ai/conversations/:id
func (h *AIHandler) GetConversation(c *gin.Context) {
	userID := c.GetUint64("user_id")
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	conv, msgs, err := h.aiService.GetConversation(id, userID)
	if err != nil {
		response.ServerError(c, err.Error())
		return
	}
	response.Success(c, gin.H{"conversation": conv, "messages": msgs})
}

// DELETE /api/ai/conversations/:id
func (h *AIHandler) DeleteConversation(c *gin.Context) {
	userID := c.GetUint64("user_id")
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	if err := h.aiService.DeleteConversation(id, userID); err != nil {
		response.ServerError(c, err.Error())
		return
	}
	response.Success(c, nil)
}

// ---- SSE 流式 ----

// sseSetup 设置 SSE 响应头，返回 flush 函数。
//
// 顺带清掉本次请求的写超时。http.Server.WriteTimeout 量的是**整条响应**的总时长，
// 对流式接口就等于给生成过程设了个硬死线：论文这类长输出会在写到一半时被切断，
// 且因为 HTTP 头早已发出，客户端只会看到流突然结束、拿不到任何错误。
// 置零后改由上游客户端的超时兜底（见 aiclient 的超时策略）。
func sseSetup(c *gin.Context) func() {
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")
	// 底层 ResponseWriter 不支持 SetWriteDeadline 时返回错误，保持原样即可，不影响 SSE
	_ = http.NewResponseController(c.Writer).SetWriteDeadline(time.Time{})
	return func() {
		c.Writer.Flush()
	}
}

// sseSend 发送一个 data 事件（JSON 载荷）。
func sseSend(c *gin.Context, payload gin.H) {
	b, _ := json.Marshal(payload)
	_, _ = c.Writer.Write([]byte("data: "))
	_, _ = c.Writer.Write(b)
	_, _ = c.Writer.Write([]byte("\n\n"))
}

// POST /api/ai/chat  (SSE)
func (h *AIHandler) Chat(c *gin.Context) {
	userID := c.GetUint64("user_id")
	var req struct {
		ConversationID uint64 `json:"conversation_id"`
		Message        string `json:"message"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Message == "" || req.ConversationID == 0 {
		response.BadRequest(c, "参数错误")
		return
	}

	flush := sseSetup(c)
	truncated, err := h.aiService.ChatStream(c.Request.Context(), userID, req.ConversationID, req.Message, func(delta string) error {
		sseSend(c, gin.H{"delta": delta})
		flush()
		return nil
	})
	if err != nil {
		sseSend(c, gin.H{"error": err.Error()})
		// SSE 已发出 200 头，用标记告知配额中间件本次失败、不要扣次数
		c.Set("ai_failed", true)
	} else {
		sseSend(c, gin.H{"done": true, "truncated": truncated})
	}
	flush()
}

// POST /api/ai/explain
func (h *AIHandler) Explain(c *gin.Context) {
	userID := c.GetUint64("user_id")
	var req struct {
		Title   string `json:"title"`
		Content string `json:"content"`
		Options string `json:"options"`
		Answer  string `json:"answer"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}
	text, err := h.aiService.Explain(c.Request.Context(), userID, req.Title, req.Content, req.Options, req.Answer)
	if err != nil {
		response.ServerError(c, err.Error())
		return
	}
	response.Success(c, gin.H{"explanation": text})
}

// ExplainStream POST /api/ai/explain-stream —— 流式题目解析（SSE）
func (h *AIHandler) ExplainStream(c *gin.Context) {
	userID := c.GetUint64("user_id")
	var req struct {
		Title   string `json:"title"`
		Content string `json:"content"`
		Options string `json:"options"`
		Answer  string `json:"answer"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}

	flush := sseSetup(c)
	truncated, err := h.aiService.ExplainStream(c.Request.Context(), userID, req.Title, req.Content, req.Options, req.Answer, func(delta string) error {
		sseSend(c, gin.H{"delta": delta})
		flush()
		return nil
	})
	if err != nil {
		sseSend(c, gin.H{"error": err.Error()})
		// SSE 已发出 200 头，用标记告知配额中间件本次失败、不要扣次数
		c.Set("ai_failed", true)
	} else {
		sseSend(c, gin.H{"done": true, "truncated": truncated})
	}
	flush()
}

// DiagnosisStream POST /api/diagnosis/ai —— 流式生成 AI 诊断建议（SSE，会员专属）。
// 无数据时错误信息以 SSE error 帧下发（流已开 200 头，业务错误不能走 JSON body）。
func (h *AIHandler) DiagnosisStream(c *gin.Context) {
	userID := c.GetUint64("user_id")
	var req struct {
		SubjectID uint64 `json:"subject_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.SubjectID == 0 {
		response.BadRequest(c, "参数错误")
		return
	}

	flush := sseSetup(c)
	truncated, err := h.aiService.DiagnosisStream(c.Request.Context(), userID, req.SubjectID, func(delta string) error {
		sseSend(c, gin.H{"delta": delta})
		flush()
		return nil
	})
	if err != nil {
		sseSend(c, gin.H{"error": err.Error()})
		// 本路由无 AIQuota 不扣额度，置位只是沿袭 SSE 失败惯例
		c.Set("ai_failed", true)
	} else {
		sseSend(c, gin.H{"done": true, "truncated": truncated})
	}
	flush()
}

// POST /api/ai/generate-questions
func (h *AIHandler) GenerateQuestions(c *gin.Context) {
	userID := c.GetUint64("user_id")
	var req struct {
		KnowledgePoint string `json:"knowledge_point"`
		Count          int    `json:"count"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.KnowledgePoint == "" {
		response.BadRequest(c, "请填写知识点")
		return
	}
	list, err := h.aiService.GenerateQuestions(c.Request.Context(), userID, req.KnowledgePoint, req.Count)
	if err != nil {
		response.ServerError(c, err.Error())
		return
	}
	response.Success(c, gin.H{"questions": list})
}

// POST /api/ai/generate-essay  (SSE)
func (h *AIHandler) GenerateEssay(c *gin.Context) {
	userID := c.GetUint64("user_id")
	var req struct {
		Topic  string `json:"topic"`
		Points string `json:"points"` // 选填: 用户指定的论点/要点
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Topic == "" {
		response.BadRequest(c, "请填写论文主题")
		return
	}
	flush := sseSetup(c)
	truncated, err := h.aiService.GenerateEssayStream(c.Request.Context(), userID, req.Topic, strings.TrimSpace(req.Points), func(delta string) error {
		sseSend(c, gin.H{"delta": delta})
		flush()
		return nil
	})
	if err != nil {
		sseSend(c, gin.H{"error": err.Error()})
		// SSE 已发出 200 头，用标记告知配额中间件本次失败、不要扣次数
		c.Set("ai_failed", true)
	} else {
		sseSend(c, gin.H{"done": true, "truncated": truncated})
	}
	flush()
}

// POST /api/ai/grade-essay
func (h *AIHandler) GradeEssay(c *gin.Context) {
	userID := c.GetUint64("user_id")
	var req struct {
		Content string `json:"content"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || len(req.Content) < 50 {
		response.BadRequest(c, "论文内容过短")
		return
	}
	result, err := h.aiService.GradeEssay(c.Request.Context(), userID, req.Content)
	if err != nil {
		response.ServerError(c, err.Error())
		return
	}
	response.Success(c, result)
}

// GradeCase POST /api/ai/grade-case —— AI 判分：案例题主观作答能否得分。
func (h *AIHandler) GradeCase(c *gin.Context) {
	userID := c.GetUint64("user_id")
	var req struct {
		Title       string  `json:"title"`
		Content     string  `json:"content"`
		Reference   string  `json:"reference"`   // 参考答案(analysis)
		Explanation string  `json:"explanation"` // 解析
		UserAnswer  string  `json:"user_answer" binding:"required"`
		Score       float32 `json:"score"` // 满分参考，可为0
	}
	if err := c.ShouldBindJSON(&req); err != nil || len(strings.TrimSpace(req.UserAnswer)) < 5 {
		response.BadRequest(c, "作答内容过短")
		return
	}
	result, err := h.aiService.GradeCase(c.Request.Context(), userID, req.Title, req.Content, req.Reference, req.Explanation, req.UserAnswer, req.Score)
	if err != nil {
		response.ServerError(c, err.Error())
		return
	}
	response.Success(c, result)
}
