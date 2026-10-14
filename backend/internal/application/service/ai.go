package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"software-exam/internal/domain/model"
	"software-exam/internal/domain/repository"
	"software-exam/pkg/aiclient"
)

type AIService struct {
	cfgRepo           repository.AIConfigRepository
	userCfgRepo       repository.UserAIConfigRepository
	convRepo          repository.AIConversationRepository
	msgRepo           repository.AIMessageRepository
	trialRepo         repository.AITrialUsageRepository
	logRepo           repository.AICallLogRepository
	userRepo          repository.UserRepository
	practiceQueryRepo repository.PracticeQueryRepository
}

func NewAIService(
	cfgRepo repository.AIConfigRepository,
	userCfgRepo repository.UserAIConfigRepository,
	convRepo repository.AIConversationRepository,
	msgRepo repository.AIMessageRepository,
	trialRepo repository.AITrialUsageRepository,
	logRepo repository.AICallLogRepository,
	userRepo repository.UserRepository,
	practiceQueryRepo repository.PracticeQueryRepository,
) *AIService {
	return &AIService{cfgRepo: cfgRepo, userCfgRepo: userCfgRepo, convRepo: convRepo, msgRepo: msgRepo, trialRepo: trialRepo, logRepo: logRepo, userRepo: userRepo, practiceQueryRepo: practiceQueryRepo}
}

// ErrAINotConfigured 表示管理员尚未启用/配置 AI。
var ErrAINotConfigured = errors.New("AI 功能未配置，请联系管理员在后台设置")

// ---- 系统提示词 ----

const sysTutor = `你是「软考星图」的系统架构设计师（软考高级）备考助教。回答要准确、简洁、条理清晰，多用要点与结论先行；涉及计算给出公式与步骤；不确定时明确说明。用中文回答。`

const sysExplain = `你是系统架构设计师阅卷老师。请对给定题目给出解析：先给正确答案，再逐条说明为什么正确、并排除干扰项，最后点出关联考点与易错点。用中文，条理清晰。`

const sysGenQuestion = `你是系统架构设计师命题专家。请按要求生成题目，并【只返回 JSON】，不要任何额外文字或代码块围栏。JSON 格式：
{"questions":[{"title":"题干","options":[{"label":"A","content":"选项内容","is_correct":true}],"answer":"A","explanation":"解析","knowledge_point":"知识点"}]}
选择题需 4 个选项且恰有一个 is_correct=true；答案 answer 为正确选项字母。`

const sysGenEssay = `你是系统架构设计师论文范文写手。请围绕给定主题写一篇 2000-2500 字的合格范文，结构为：摘要（约300字）、正文（项目背景与角色、核心技术论述、实施效果与不足三部分）、结尾。要结合一个具体项目，技术真实可信。用中文，Markdown 小标题分段。

直接输出范文正文本身，不要用 Markdown 代码块围栏（三个反引号那种）把整篇文章包起来 —— 围栏是给代码用的，包住全文会让读者看到 Markdown 源码而不是排版。`

const sysGradeEssay = `你是系统架构设计师论文阅卷老师，满分 75 分。请对考生论文按维度评分并【只返回 JSON】，不要额外文字或代码块围栏。JSON 格式：
{"total":60,"dimensions":[{"name":"切题","score":13,"full":15,"comment":"评语"}],"summary":"总评","suggestions":["改进建议1","改进建议2"]}
维度建议包含：切题(15)、结构完整(15)、技术深度(20)、真实性(15)、字数与表达(10)。`

const sysGradeCase = `你是系统架构设计师案例分析阅卷老师。请依据题干与参考答案，对考生作答判断能否得分并评分，【只返回 JSON】，不要额外文字或代码块围栏。JSON 格式：
{"total":8,"full":15,"dimensions":[{"name":"问题1","score":3,"full":5,"comment":"评语"},{"name":"问题2","score":5,"full":10,"comment":"评语"}],"summary":"总评","suggestions":["改进建议1","改进建议2"]}
要求：dimensions 按题干小问逐条给出得分与评语；题干未分小问时只给一条 name 为「本题」。总分 total 不得超过 full。请严格对照参考答案评分，客观指出哪些得分点答到了、哪些漏了。`

// resolveConfig 按用户解析生效配置。
//
// 会员自定义配置是「全有或全无」：base_url / api_key / model 必须同时自带才生效，
// 缺任一项则整体忽略、回落到平台配置。绝不逐字段回退 —— 否则会员只填 base_url、
// api_key 留空，平台的 key 就会被带着 Authorization 头发到用户指定的任意服务器。
// 会员配置完整时不受全局开关限制（用的是自己的额度）。
func (s *AIService) resolveConfig(userID uint64) (aiclient.Config, error) {
	g, err := s.cfgRepo.Get()
	if err != nil {
		return aiclient.Config{}, err
	}

	if userID != 0 {
		if u, uerr := s.userCfgRepo.GetByUser(userID); uerr == nil && userConfigUsable(u) {
			maxTok := u.MaxTokens
			if maxTok <= 0 {
				maxTok = g.MaxTokens
			}
			if maxTok <= 0 {
				maxTok = 2048
			}
			return aiclient.Config{
				BaseURL:     u.BaseURL,
				APIKey:      u.APIKey,
				Model:       u.Model,
				Temperature: u.Temperature,
				MaxTokens:   maxTok,
				Restricted:  true,
			}, nil
		}
	}

	if g.Enabled != 1 || g.BaseURL == "" || g.APIKey == "" || g.Model == "" {
		return aiclient.Config{}, ErrAINotConfigured
	}
	maxTok := g.MaxTokens
	if maxTok <= 0 {
		maxTok = 2048
	}
	return aiclient.Config{
		BaseURL:     g.BaseURL,
		APIKey:      g.APIKey,
		Model:       g.Model,
		Temperature: g.Temperature,
		MaxTokens:   maxTok,
	}, nil
}

// userConfigUsable 会员自定义配置是否可独立成套使用。
func userConfigUsable(u *model.UserAIConfig) bool {
	return u != nil && u.Enabled == 1 && u.BaseURL != "" && u.APIKey != "" && u.Model != ""
}

// Enabled 供 /ai/status（按用户，考虑会员自定义配置）。
func (s *AIService) Enabled(userID uint64) bool {
	_, err := s.resolveConfig(userID)
	return err == nil
}

// UsesOwnConfig 用户是否用的是自己的 key（自带 key 不占用平台免费额度）。
func (s *AIService) UsesOwnConfig(userID uint64) bool {
	u, err := s.userCfgRepo.GetByUser(userID)
	return err == nil && userConfigUsable(u)
}

// TrialQuota 返回非会员的免费试用 (已用, 总额度)。
func (s *AIService) TrialQuota(userID uint64) (used int, limit int) {
	if cfg, err := s.cfgRepo.Get(); err == nil {
		limit = cfg.FreeTrialLimit
	}
	if limit <= 0 {
		return 0, 0
	}
	used, _ = s.trialRepo.GetUsed(userID)
	return used, limit
}

// ConsumeTrial 扣减一次免费额度，失败静默忽略（不阻断已完成的响应）。
func (s *AIService) ConsumeTrial(userID uint64) {
	s.trialRepo.Increment(userID)
}

// ---- 调用日志 ----

// callLog 记录一次调用。失败也记 —— 排查上游抽风时，只有成功记录看不出问题。
// 落库出错一律吞掉：日志是旁路，不能让它把已经成功的回答变成报错。
func (s *AIService) callLog(userID uint64, scene string, cfg aiclient.Config, u aiclient.Usage, start time.Time, estimated bool, err error) {
	if s.logRepo == nil {
		return
	}
	l := &model.AICallLog{
		UserID:           userID,
		Scene:            scene,
		Model:            cfg.Model,
		PromptTokens:     u.PromptTokens,
		CompletionTokens: u.CompletionTokens,
		TotalTokens:      u.TotalTokens,
		DurationMs:       int(time.Since(start).Milliseconds()),
		Success:          1,
		CreatedAt:        time.Now(),
	}
	// Restricted 只在会员自带配置时置位，正是「这次不花平台的钱」的判据
	if cfg.Restricted {
		l.OwnKey = 1
	}
	if estimated {
		l.Estimated = 1
	}
	if err != nil {
		l.Success = 0
		l.ErrMsg = truncateRunes(err.Error(), 120)
	}
	_ = s.logRepo.Create(l)
}

// logStream 记录一次流式调用。上游没给 usage（绝大多数情况）时退回字符估算，
// 并把 estimated 置位，避免估算值被当成真实计数进账单。
func (s *AIService) logStream(userID uint64, scene string, cfg aiclient.Config, messages []aiclient.Message, output string, u aiclient.Usage, start time.Time, err error) {
	if u.TotalTokens == 0 {
		s.callLog(userID, scene, cfg, estimateUsage(messages, output), start, true, err)
		return
	}
	s.callLog(userID, scene, cfg, u, start, false, err)
}

// estimateUsage 上游没返回 usage 时按字符数粗估。
// 中英文混排下 1 token ≈ 1.5 字符是够用的量级近似 —— 这个值只用来看趋势和找异常大户，
// 不作账单依据，所以调用方必须同时把 estimated 置位。
func estimateUsage(messages []aiclient.Message, output string) aiclient.Usage {
	in := 0
	for _, m := range messages {
		in += len([]rune(m.Content))
	}
	p := int(float64(in) / 1.5)
	c := int(float64(len([]rune(output))) / 1.5)
	return aiclient.Usage{PromptTokens: p, CompletionTokens: c, TotalTokens: p + c}
}

// ---- 用量统计（管理员） ----

type AIUsageStats struct {
	Days     int                     `json:"days"`
	Calls    int64                   `json:"calls"`
	Tokens   int64                   `json:"tokens"`
	Failed   int64                   `json:"failed"`
	Users    int64                   `json:"users"`
	ByScene  []repository.AICallStat `json:"by_scene"`
	ByDay    []repository.AICallStat `json:"by_day"`
	TopUsers []repository.AICallStat `json:"top_users"`
}

func (s *AIService) UsageStats(days int) (*AIUsageStats, error) {
	if days <= 0 || days > 90 {
		days = 7
	}
	calls, tokens, failed, users, err := s.logRepo.Summary(days)
	if err != nil {
		return nil, err
	}
	st := &AIUsageStats{Days: days, Calls: calls, Tokens: tokens, Failed: failed, Users: users}
	st.ByScene, _ = s.logRepo.StatByScene(days)
	st.ByDay, _ = s.logRepo.StatByDay(days)
	st.TopUsers, _ = s.logRepo.TopUsers(days, 10)
	// TopUsers 的 Key 是 user_id，补成用户名 —— 面板上只有数字 ID 没法定位到人。
	// 查不到（已注销）就留空，不影响整份统计。
	for i := range st.TopUsers {
		id, convErr := strconv.ParseUint(st.TopUsers[i].Key, 10, 64)
		if convErr != nil {
			continue
		}
		if u, uerr := s.userRepo.FindByID(id); uerr == nil {
			st.TopUsers[i].Name = u.Username
		}
	}
	return st, nil
}

func (s *AIService) ListCallLogs(scene string, page, pageSize int) ([]model.AICallLog, int64, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 20
	}
	return s.logRepo.List(scene, page, pageSize)
}

// ---- 配置管理（管理员） ----

// AIConfigGlobalView 管理员全局 AI 配置出参（脱敏，不含明文 key）。
// free_trial_limit 不加 omitempty：值为 0 也要输出，否则管理端表单读不到 0 会被重置。
// key_set/key_masked 恒输出（true/false、空串），前端据此判断是否已配置密钥。
type AIConfigGlobalView struct {
	BaseURL        string  `json:"base_url"`
	Model          string  `json:"model"`
	Temperature    float64 `json:"temperature"`
	MaxTokens      int     `json:"max_tokens"`
	Enabled        int8    `json:"enabled"`
	FreeTrialLimit int     `json:"free_trial_limit"`
	KeySet         bool    `json:"key_set"`
	KeyMasked      string  `json:"key_masked"`
}

// AIConfigUserView 会员自定义 AI 配置出参（脱敏）。字段与全局视图一致，
// 但不含 free_trial_limit —— 试用额度是全局配置属性，会员配置没有这个字段。
// 用两个结构体而非一个带 omitempty：避免 free_trial_limit=0 时被省略把
// 全局表单与会员表单的行为搅在一起。
type AIConfigUserView struct {
	BaseURL     string  `json:"base_url"`
	Model       string  `json:"model"`
	Temperature float64 `json:"temperature"`
	MaxTokens   int     `json:"max_tokens"`
	Enabled     int8    `json:"enabled"`
	KeySet      bool    `json:"key_set"`
	KeyMasked   string  `json:"key_masked"`
}

// GetConfigMasked 返回脱敏配置（不含明文 key）。
func (s *AIService) GetConfigMasked() (*AIConfigGlobalView, error) {
	c, err := s.cfgRepo.Get()
	if err != nil {
		return nil, err
	}
	return &AIConfigGlobalView{
		BaseURL:        c.BaseURL,
		Model:          c.Model,
		Temperature:    c.Temperature,
		MaxTokens:      c.MaxTokens,
		Enabled:        c.Enabled,
		FreeTrialLimit: c.FreeTrialLimit,
		KeySet:         c.APIKey != "",
		KeyMasked:      maskKey(c.APIKey),
	}, nil
}

// UpdateConfig 更新配置；apiKey 为空字符串则保留原值。
func (s *AIService) UpdateConfig(baseURL, apiKey, mdl string, temperature float64, maxTokens int, enabled int8, freeTrialLimit int) error {
	c, err := s.cfgRepo.Get()
	if err != nil {
		return err
	}
	c.BaseURL = strings.TrimSpace(baseURL)
	c.Model = strings.TrimSpace(mdl)
	c.Temperature = temperature
	c.MaxTokens = maxTokens
	c.Enabled = enabled
	if freeTrialLimit < 0 {
		freeTrialLimit = 0
	}
	c.FreeTrialLimit = freeTrialLimit
	// 清洗 key：去空白、包裹引号、误粘贴的 "Bearer " 前缀
	key := strings.TrimSpace(apiKey)
	key = strings.Trim(key, "\"'`")
	key = strings.TrimSpace(strings.TrimPrefix(key, "Bearer "))
	key = strings.TrimSpace(strings.TrimPrefix(key, "bearer "))
	if key != "" {
		c.APIKey = key
	}
	c.UpdatedAt = time.Now()
	return s.cfgRepo.Update(c)
}

// TestConfig 用全局配置做一次最小连通性测试（管理员）。
func (s *AIService) TestConfig() (string, error) {
	cfg, err := s.resolveConfig(0)
	if err != nil {
		return "", err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	start := time.Now()
	out, usage, err := aiclient.ChatCompletion(ctx, cfg, []aiclient.Message{
		{Role: "system", Content: "你是测试助手，只回复：ok"},
		{Role: "user", Content: "回复 ok"},
	})
	s.callLog(0, "test", cfg, usage, start, false, err)
	return out, err
}

// ---- 会员自定义配置 ----

// GetUserConfigMasked 返回会员脱敏配置（不含明文 key）。
func (s *AIService) GetUserConfigMasked(userID uint64) (*AIConfigUserView, error) {
	c, err := s.userCfgRepo.GetByUser(userID)
	if err != nil {
		return nil, err
	}
	return &AIConfigUserView{
		BaseURL:     c.BaseURL,
		Model:       c.Model,
		Temperature: c.Temperature,
		MaxTokens:   c.MaxTokens,
		Enabled:     c.Enabled,
		KeySet:      c.APIKey != "",
		KeyMasked:   maskKey(c.APIKey),
	}, nil
}

// UpdateUserConfig 更新会员配置；apiKey 为空字符串则保留原值。
func (s *AIService) UpdateUserConfig(userID uint64, baseURL, apiKey, mdl string, temperature float64, maxTokens int, enabled int8) error {
	c, err := s.userCfgRepo.GetByUser(userID)
	if err != nil {
		return err
	}
	c.UserID = userID
	c.BaseURL = strings.TrimSpace(baseURL)
	c.Model = strings.TrimSpace(mdl)
	c.Temperature = temperature
	c.MaxTokens = maxTokens
	c.Enabled = enabled
	key := strings.TrimSpace(apiKey)
	key = strings.Trim(key, "\"'`")
	key = strings.TrimSpace(strings.TrimPrefix(key, "Bearer "))
	key = strings.TrimSpace(strings.TrimPrefix(key, "bearer "))
	if key != "" {
		c.APIKey = key
	}
	// 启用时必须三项齐全：不允许只填 base_url 而借用平台的 key
	if c.Enabled == 1 && !userConfigUsable(c) {
		return errors.New("启用自定义 AI 需同时填写接口地址、API Key 和模型名")
	}
	if c.BaseURL != "" {
		if err := aiclient.ValidateUserBaseURL(c.BaseURL); err != nil {
			return err
		}
	}
	now := time.Now()
	if c.CreatedAt.IsZero() {
		c.CreatedAt = now
	}
	c.UpdatedAt = now
	return s.userCfgRepo.Upsert(c)
}

// TestUserConfig 用会员生效配置做一次连通性测试。
func (s *AIService) TestUserConfig(userID uint64) (string, error) {
	cfg, err := s.resolveConfig(userID)
	if err != nil {
		return "", err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	start := time.Now()
	out, usage, err := aiclient.ChatCompletion(ctx, cfg, []aiclient.Message{
		{Role: "system", Content: "你是测试助手，只回复：ok"},
		{Role: "user", Content: "回复 ok"},
	})
	s.callLog(userID, "test", cfg, usage, start, false, err)
	return out, err
}

// ---- 会话（AI问答） ----

func (s *AIService) ListConversations(userID uint64) ([]model.AIConversation, error) {
	return s.convRepo.FindByUser(userID)
}

func (s *AIService) CreateConversation(userID uint64, title string) (*model.AIConversation, error) {
	if title == "" {
		title = "新对话"
	}
	conv := &model.AIConversation{UserID: userID, Title: title, CreatedAt: time.Now(), UpdatedAt: time.Now()}
	if err := s.convRepo.Create(conv); err != nil {
		return nil, err
	}
	return conv, nil
}

func (s *AIService) GetConversation(id, userID uint64) (*model.AIConversation, []model.AIMessage, error) {
	conv, err := s.convRepo.FindByID(id)
	if err != nil {
		return nil, nil, err
	}
	if conv.UserID != userID {
		return nil, nil, errors.New("无权访问该会话")
	}
	msgs, err := s.msgRepo.FindByConversation(id)
	return conv, msgs, err
}

func (s *AIService) DeleteConversation(id, userID uint64) error {
	return s.convRepo.Delete(id, userID)
}

// ChatStream 处理一轮流式对话：落用户消息 → 流式回传 → 落 assistant 消息。
// 返回值 truncated 表示上游因长度上限截断（见 aiclient.StreamResult.Truncated），
// 此时不算错误，但调用方要告知用户内容不完整。
func (s *AIService) ChatStream(ctx context.Context, userID, convID uint64, userMsg string, onDelta func(string) error) (bool, error) {
	cfg, err := s.resolveConfig(userID)
	if err != nil {
		return false, err
	}
	conv, err := s.convRepo.FindByID(convID)
	if err != nil || conv.UserID != userID {
		return false, errors.New("会话不存在")
	}

	history, _ := s.msgRepo.FindByConversation(convID)
	messages := []aiclient.Message{{Role: "system", Content: sysTutor}}
	for _, m := range history {
		messages = append(messages, aiclient.Message{Role: m.Role, Content: m.Content})
	}
	messages = append(messages, aiclient.Message{Role: "user", Content: userMsg})

	// 落用户消息
	_ = s.msgRepo.Create(&model.AIMessage{ConversationID: convID, Role: "user", Content: userMsg, CreatedAt: time.Now()})
	// 首条消息用其作标题
	if conv.Title == "" || conv.Title == "新对话" {
		conv.Title = truncateRunes(userMsg, 20)
	}
	conv.UpdatedAt = time.Now()
	_ = s.convRepo.Update(conv)

	var sb strings.Builder
	start := time.Now()
	res, streamErr := aiclient.ChatCompletionStream(ctx, cfg, messages, func(delta string) error {
		sb.WriteString(delta)
		return onDelta(delta)
	})
	s.logStream(userID, "chat", cfg, messages, sb.String(), res.Usage, start, streamErr)

	// 落 assistant 消息（即便中途出错也保存已生成部分）
	if sb.Len() > 0 {
		_ = s.msgRepo.Create(&model.AIMessage{ConversationID: convID, Role: "assistant", Content: sb.String(), CreatedAt: time.Now()})
	}
	return res.Truncated, streamErr
}

// ---- 题目解析 ----

func (s *AIService) Explain(ctx context.Context, userID uint64, title, content, options, answer string) (string, error) {
	cfg, err := s.resolveConfig(userID)
	if err != nil {
		return "", err
	}
	prompt := fmt.Sprintf("题干：%s\n\n%s\n\n选项：\n%s\n\n正确答案：%s\n\n请给出解析。", title, content, options, answer)
	start := time.Now()
	out, usage, err := aiclient.ChatCompletion(ctx, cfg, []aiclient.Message{
		{Role: "system", Content: sysExplain},
		{Role: "user", Content: prompt},
	})
	s.callLog(userID, "explain", cfg, usage, start, false, err)
	return out, err
}

// ExplainStream 流式题目解析，逐块回调。
// 返回值 truncated 含义同 ChatStream。
func (s *AIService) ExplainStream(ctx context.Context, userID uint64, title, content, options, answer string, onDelta func(string) error) (bool, error) {
	cfg, err := s.resolveConfig(userID)
	if err != nil {
		return false, err
	}
	prompt := fmt.Sprintf("题干：%s\n\n%s\n\n选项：\n%s\n\n正确答案：%s\n\n请给出解析。", title, content, options, answer)
	messages := []aiclient.Message{
		{Role: "system", Content: sysExplain},
		{Role: "user", Content: prompt},
	}
	var sb strings.Builder
	start := time.Now()
	res, streamErr := aiclient.ChatCompletionStream(ctx, cfg, messages, func(delta string) error {
		sb.WriteString(delta)
		return onDelta(delta)
	})
	s.logStream(userID, "explain", cfg, messages, sb.String(), res.Usage, start, streamErr)
	return res.Truncated, streamErr
}

// ---- AI 出题 ----

type GenOption struct {
	Label     string `json:"label"`
	Content   string `json:"content"`
	IsCorrect bool   `json:"is_correct"`
}

type GenQuestion struct {
	Title          string      `json:"title"`
	Options        []GenOption `json:"options"`
	Answer         string      `json:"answer"`
	Explanation    string      `json:"explanation"`
	KnowledgePoint string      `json:"knowledge_point"`
}

func (s *AIService) GenerateQuestions(ctx context.Context, userID uint64, knowledgePoint string, count int) ([]GenQuestion, error) {
	cfg, err := s.resolveConfig(userID)
	if err != nil {
		return nil, err
	}
	if count <= 0 || count > 10 {
		count = 3
	}
	prompt := fmt.Sprintf("知识点：%s\n题型：单项选择题\n数量：%d\n请生成题目。", knowledgePoint, count)
	start := time.Now()
	raw, usage, err := aiclient.ChatCompletion(ctx, cfg, []aiclient.Message{
		{Role: "system", Content: sysGenQuestion},
		{Role: "user", Content: prompt},
	})
	s.callLog(userID, "generate-questions", cfg, usage, start, false, err)
	if err != nil {
		return nil, err
	}
	var out struct {
		Questions []GenQuestion `json:"questions"`
	}
	jsonStr := extractJSON(raw)
	if err := json.Unmarshal([]byte(jsonStr), &out); err != nil {
		// 顶层数组形态（[...] 而非 {questions:[...]}）：直接当作题目列表。
		var list []GenQuestion
		if e2 := json.Unmarshal([]byte(jsonStr), &list); e2 == nil && len(list) > 0 {
			return list, nil
		}
		s.callLog(userID, "generate-questions-parse", cfg, usage, start, false, fmt.Errorf("解析失败 raw=%s: %v", truncateRunes(raw, 200), err))
		return nil, fmt.Errorf("解析生成结果失败：%v", err)
	}
	// 兜底：对象解出来但 questions 为空，再试一次顶层数组。
	if len(out.Questions) == 0 {
		var list []GenQuestion
		if err := json.Unmarshal([]byte(jsonStr), &list); err == nil && len(list) > 0 {
			return list, nil
		}
		s.callLog(userID, "generate-questions-empty", cfg, usage, start, false, fmt.Errorf("解析成功但 questions 为空 raw=%s", truncateRunes(raw, 200)))
		return nil, fmt.Errorf("未生成题目，请调整知识点重试")
	}
	return out.Questions, nil
}

// ---- AI 生成论文（流式） ----

// points 是用户自选的论点/要点, 可以为空; 给了就要求逐一覆盖, 否则 AI 容易自说自话
// 写一篇与该主题无关的通用范文。
// 返回值 truncated 含义同 ChatStream —— 论文最长, 是最容易被 max_tokens 截断的场景。
func (s *AIService) GenerateEssayStream(ctx context.Context, userID uint64, topic, points string, onDelta func(string) error) (bool, error) {
	cfg, err := s.resolveConfig(userID)
	if err != nil {
		return false, err
	}
	prompt := "论文主题：" + topic
	if points != "" {
		prompt += "\n\n必须在正文中逐一论述以下论点/要点，不得遗漏：" + points
	}
	messages := []aiclient.Message{
		{Role: "system", Content: sysGenEssay},
		{Role: "user", Content: prompt},
	}
	var sb strings.Builder
	start := time.Now()
	res, streamErr := aiclient.ChatCompletionStream(ctx, cfg, messages, func(delta string) error {
		sb.WriteString(delta)
		return onDelta(delta)
	})
	s.logStream(userID, "generate-essay", cfg, messages, sb.String(), res.Usage, start, streamErr)
	return res.Truncated, streamErr
}

// ---- AI 批阅论文 ----

// GradeEssay/GradeCase 有意保持 map[string]interface{}：这是 LLM 输出经 extractJSON
// 原样透传的 JSON，形状由大模型决定（dimensions 数量/字段随题面变化），
// 前端已经按动态结构消费。硬套 DTO 反而要在模型与前端之间补一层不稳定映射。
// 除此之外 service 层已无手拼 untyped map 的出参。
func (s *AIService) GradeEssay(ctx context.Context, userID uint64, content string) (map[string]interface{}, error) {
	cfg, err := s.resolveConfig(userID)
	if err != nil {
		return nil, err
	}
	start := time.Now()
	raw, usage, err := aiclient.ChatCompletion(ctx, cfg, []aiclient.Message{
		{Role: "system", Content: sysGradeEssay},
		{Role: "user", Content: "考生论文：\n" + content},
	})
	s.callLog(userID, "grade-essay", cfg, usage, start, false, err)
	if err != nil {
		return nil, err
	}
	var out map[string]interface{}
	if err := json.Unmarshal([]byte(extractJSON(raw)), &out); err != nil {
		s.callLog(userID, "grade-essay-parse", cfg, usage, start, false, fmt.Errorf("解析失败 raw=%s: %v", truncateRunes(raw, 200), err))
		return nil, fmt.Errorf("解析评分结果失败：%v", err)
	}
	return out, nil
}

// GradeCase AI 判分：案例分析题（主观题）作答能否得分。
// fullScore<=0 时视为题面未标注满分，交由模型按题干判断。
func (s *AIService) GradeCase(ctx context.Context, userID uint64, title, content, reference, explanation, userAnswer string, fullScore float32) (map[string]interface{}, error) {
	cfg, err := s.resolveConfig(userID)
	if err != nil {
		return nil, err
	}
	var sb strings.Builder
	sb.WriteString("题干：\n" + title)
	if content != "" && content != title {
		sb.WriteString("\n" + content)
	}
	if reference != "" {
		sb.WriteString("\n\n参考答案：\n" + reference)
	}
	if explanation != "" {
		sb.WriteString("\n\n解析：\n" + explanation)
	}
	if fullScore > 0 {
		sb.WriteString(fmt.Sprintf("\n\n本题满分：%g 分", fullScore))
	}
	sb.WriteString("\n\n考生作答：\n" + userAnswer)
	sb.WriteString("\n\n请严格对照参考答案评分，只返回 JSON。")

	start := time.Now()
	raw, usage, err := aiclient.ChatCompletion(ctx, cfg, []aiclient.Message{
		{Role: "system", Content: sysGradeCase},
		{Role: "user", Content: sb.String()},
	})
	s.callLog(userID, "grade-case", cfg, usage, start, false, err)
	if err != nil {
		return nil, err
	}
	var out map[string]interface{}
	if err := json.Unmarshal([]byte(extractJSON(raw)), &out); err != nil {
		s.callLog(userID, "grade-case-parse", cfg, usage, start, false, fmt.Errorf("解析失败 raw=%s: %v", truncateRunes(raw, 200), err))
		return nil, fmt.Errorf("解析判分结果失败：%v", err)
	}
	return out, nil
}

// ---- helpers ----

func maskKey(key string) string {
	if key == "" {
		return ""
	}
	if len(key) <= 8 {
		return "****"
	}
	return key[:4] + "****" + key[len(key)-4:]
}

// extractJSON 从可能带围栏/前后缀的文本中抽出首个完整的 JSON 值（对象或数组）。
// 用括号配平逐字符扫描，避免贪婪正则把模型附加的多段 JSON / 文字一起吞进来，
// 从而出现 "invalid character ',' after top-level value" 之类的解析错误。
// 字符串字面量内部的大括号不计入配平。
func extractJSON(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "```json")
	s = strings.TrimPrefix(s, "```")
	s = strings.TrimSuffix(s, "```")

	// 取首个对象或数组起始符作为起点。
	start := -1
	for i := 0; i < len(s); i++ {
		if s[i] == '{' || s[i] == '[' {
			start = i
			break
		}
	}
	if start < 0 {
		return s
	}
	depth := 0
	inStr := false
	esc := false
	for i := start; i < len(s); i++ {
		c := s[i]
		if esc {
			esc = false
			continue
		}
		switch c {
		case '\\':
			esc = true
		case '"':
			inStr = !inStr
		case '{', '[':
			if !inStr {
				depth++
			}
		case '}', ']':
			if !inStr {
				depth--
				if depth == 0 {
					return s[start : i+1]
				}
			}
		}
	}
	return s[start:]
}

func truncateRunes(s string, n int) string {
	r := []rune(strings.TrimSpace(s))
	if len(r) <= n {
		return string(r)
	}
	return string(r[:n])
}
