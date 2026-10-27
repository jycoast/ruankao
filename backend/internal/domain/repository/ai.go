package repository

import (
	"time"

	"software-exam/internal/domain/model"

	"gorm.io/gorm"
)

type AIConfigRepository interface {
	Get() (*model.AIConfig, error)
	Update(cfg *model.AIConfig) error
}

type AIConfigRepositoryImpl struct {
	db *gorm.DB
}

func NewAIConfigRepository(db *gorm.DB) AIConfigRepository {
	return &AIConfigRepositoryImpl{db: db}
}

func (r *AIConfigRepositoryImpl) Get() (*model.AIConfig, error) {
	var cfg model.AIConfig
	// 固定单行 id=1；不存在则返回空配置
	err := r.db.First(&cfg, 1).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return &model.AIConfig{ID: 1}, nil
		}
		return nil, err
	}
	return &cfg, nil
}

func (r *AIConfigRepositoryImpl) Update(cfg *model.AIConfig) error {
	cfg.ID = 1
	return r.db.Save(cfg).Error
}

// ---- User AI Config（会员自定义） ----

type UserAIConfigRepository interface {
	GetByUser(userID uint64) (*model.UserAIConfig, error)
	Upsert(cfg *model.UserAIConfig) error
}

type UserAIConfigRepositoryImpl struct {
	db *gorm.DB
}

func NewUserAIConfigRepository(db *gorm.DB) UserAIConfigRepository {
	return &UserAIConfigRepositoryImpl{db: db}
}

// GetByUser 返回该用户的配置；不存在则返回空配置（ID=0），调用方据此判断是否已配置。
func (r *UserAIConfigRepositoryImpl) GetByUser(userID uint64) (*model.UserAIConfig, error) {
	var cfg model.UserAIConfig
	err := r.db.Where("user_id = ?", userID).First(&cfg).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return &model.UserAIConfig{UserID: userID}, nil
		}
		return nil, err
	}
	return &cfg, nil
}

// Upsert 按 user_id 单行插入或更新（find-then-save，规避 OnConflict）。
func (r *UserAIConfigRepositoryImpl) Upsert(cfg *model.UserAIConfig) error {
	var existing model.UserAIConfig
	err := r.db.Where("user_id = ?", cfg.UserID).First(&existing).Error
	if err == gorm.ErrRecordNotFound {
		return r.db.Create(cfg).Error
	}
	if err != nil {
		return err
	}
	cfg.ID = existing.ID
	return r.db.Save(cfg).Error
}

// ---- AI Trial Usage（非会员免费试用计数） ----

type AITrialUsageRepository interface {
	GetUsed(userID uint64) (int, error)
	// TryIncrement 原子占用一次额度：used_count < limit 才 +1。
	// 返回 false 表示额度已满（limit<=0 时直接视为会员专属，不占用）。
	TryIncrement(userID uint64, limit int) (bool, error)
	// Decrement 归还一次额度（上游失败时）。
	Decrement(userID uint64) error
}

type AITrialUsageRepositoryImpl struct {
	db *gorm.DB
}

func NewAITrialUsageRepository(db *gorm.DB) AITrialUsageRepository {
	return &AITrialUsageRepositoryImpl{db: db}
}

func (r *AITrialUsageRepositoryImpl) GetUsed(userID uint64) (int, error) {
	var u model.AITrialUsage
	err := r.db.Where("user_id = ?", userID).First(&u).Error
	if err == gorm.ErrRecordNotFound {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	return u.UsedCount, nil
}

// TryIncrement 把「是否还有额度」写进 UPDATE 的 WHERE，
// 避免先 GetUsed 再 Increment 时并发请求同时通过检查。
func (r *AITrialUsageRepositoryImpl) TryIncrement(userID uint64, limit int) (bool, error) {
	if limit <= 0 {
		return false, nil
	}
	ok, err := r.bumpUnderLimit(userID, limit)
	if err != nil || ok {
		return ok, err
	}

	// 0 行有两种可能：额度已满（行存在），或首次使用（行不存在）。
	// 用一次轻量 COUNT 区分，避免额度已满时反复撞唯一索引。
	var n int64
	if err := r.db.Model(&model.AITrialUsage{}).Where("user_id = ?", userID).Count(&n).Error; err != nil {
		return false, err
	}
	if n > 0 {
		return false, nil
	}
	if err := r.db.Create(&model.AITrialUsage{UserID: userID, UsedCount: 1}).Error; err != nil {
		// 并发下别人刚插入：退化为条件自增，额度上限仍由 WHERE 兜住
		return r.bumpUnderLimit(userID, limit)
	}
	return true, nil
}

func (r *AITrialUsageRepositoryImpl) bumpUnderLimit(userID uint64, limit int) (bool, error) {
	res := r.db.Model(&model.AITrialUsage{}).
		Where("user_id = ? AND used_count < ?", userID, limit).
		UpdateColumn("used_count", gorm.Expr("used_count + 1"))
	if res.Error != nil {
		return false, res.Error
	}
	return res.RowsAffected > 0, nil
}

// Decrement 归还额度；used_count 不会降到负数。
func (r *AITrialUsageRepositoryImpl) Decrement(userID uint64) error {
	return r.db.Model(&model.AITrialUsage{}).
		Where("user_id = ? AND used_count > 0", userID).
		UpdateColumn("used_count", gorm.Expr("used_count - 1")).Error
}

// ---- AI Call Log ----

// AICallStat 一个分组维度上的用量汇总。
// TopUsers 场景下 Key 是 user_id，Name 由上层补成用户名 —— 面板上光看数字 ID 没法用。
type AICallStat struct {
	Key    string `gorm:"column:k" json:"key"`
	Name   string `gorm:"-" json:"name,omitempty"`
	Calls  int64  `gorm:"column:calls" json:"calls"`
	Tokens int64  `gorm:"column:tokens" json:"tokens"`
	Failed int64  `gorm:"column:failed" json:"failed"`
}

type AICallLogRepository interface {
	Create(log *model.AICallLog) error
	// List 按时间倒序分页，scene 为空表示不过滤。
	List(scene string, page, pageSize int) ([]model.AICallLog, int64, error)
	// StatByScene / StatByDay 只统计最近 days 天。
	StatByScene(days int) ([]AICallStat, error)
	StatByDay(days int) ([]AICallStat, error)
	// TopUsers 按 token 消耗排序，只看平台出钱的调用（own_key=0）。
	TopUsers(days, limit int) ([]AICallStat, error)
	Summary(days int) (calls int64, tokens int64, failed int64, users int64, err error)
}

type AICallLogRepositoryImpl struct {
	db *gorm.DB
}

func NewAICallLogRepository(db *gorm.DB) AICallLogRepository {
	return &AICallLogRepositoryImpl{db: db}
}

func (r *AICallLogRepositoryImpl) Create(log *model.AICallLog) error {
	return r.db.Create(log).Error
}

func (r *AICallLogRepositoryImpl) List(scene string, page, pageSize int) ([]model.AICallLog, int64, error) {
	q := r.db.Model(&model.AICallLog{})
	if scene != "" {
		q = q.Where("scene = ?", scene)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var list []model.AICallLog
	err := q.Order("id DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&list).Error
	return list, total, err
}

// sinceDays 统一时间窗口下界；非正数按 7 天。
func sinceDays(days int) time.Time {
	if days <= 0 {
		days = 7
	}
	return time.Now().AddDate(0, 0, -days+1).Truncate(24 * time.Hour)
}

func (r *AICallLogRepositoryImpl) StatByScene(days int) ([]AICallStat, error) {
	var out []AICallStat
	err := r.db.Model(&model.AICallLog{}).
		Select("scene AS k, COUNT(*) AS calls, COALESCE(SUM(total_tokens),0) AS tokens, SUM(success = 0) AS failed").
		Where("created_at >= ?", sinceDays(days)).
		Group("scene").Order("calls DESC").Scan(&out).Error
	return out, err
}

func (r *AICallLogRepositoryImpl) StatByDay(days int) ([]AICallStat, error) {
	var out []AICallStat
	err := r.db.Model(&model.AICallLog{}).
		Select("DATE(created_at) AS k, COUNT(*) AS calls, COALESCE(SUM(total_tokens),0) AS tokens, SUM(success = 0) AS failed").
		Where("created_at >= ?", sinceDays(days)).
		Group("DATE(created_at)").Order("k ASC").Scan(&out).Error
	return out, err
}

func (r *AICallLogRepositoryImpl) TopUsers(days, limit int) ([]AICallStat, error) {
	if limit <= 0 {
		limit = 10
	}
	var out []AICallStat
	err := r.db.Model(&model.AICallLog{}).
		Select("user_id AS k, COUNT(*) AS calls, COALESCE(SUM(total_tokens),0) AS tokens, SUM(success = 0) AS failed").
		Where("created_at >= ? AND own_key = 0", sinceDays(days)).
		Group("user_id").Order("tokens DESC, calls DESC").Limit(limit).Scan(&out).Error
	return out, err
}

func (r *AICallLogRepositoryImpl) Summary(days int) (int64, int64, int64, int64, error) {
	var row struct {
		Calls  int64
		Tokens int64
		Failed int64
		Users  int64
	}
	err := r.db.Model(&model.AICallLog{}).
		Select("COUNT(*) AS calls, COALESCE(SUM(total_tokens),0) AS tokens, SUM(success = 0) AS failed, COUNT(DISTINCT user_id) AS users").
		Where("created_at >= ?", sinceDays(days)).
		Scan(&row).Error
	return row.Calls, row.Tokens, row.Failed, row.Users, err
}

// ---- AI Conversation ----

type AIConversationRepository interface {
	Create(conv *model.AIConversation) error
	FindByUser(userID uint64) ([]model.AIConversation, error)
	FindByID(id uint64) (*model.AIConversation, error)
	Update(conv *model.AIConversation) error
	Delete(id, userID uint64) error
}

type AIConversationRepositoryImpl struct {
	db *gorm.DB
}

func NewAIConversationRepository(db *gorm.DB) AIConversationRepository {
	return &AIConversationRepositoryImpl{db: db}
}

func (r *AIConversationRepositoryImpl) Create(conv *model.AIConversation) error {
	return r.db.Create(conv).Error
}

func (r *AIConversationRepositoryImpl) FindByUser(userID uint64) ([]model.AIConversation, error) {
	var list []model.AIConversation
	err := r.db.Where("user_id = ?", userID).Order("updated_at DESC").Find(&list).Error
	return list, err
}

func (r *AIConversationRepositoryImpl) FindByID(id uint64) (*model.AIConversation, error) {
	var conv model.AIConversation
	err := r.db.First(&conv, id).Error
	return &conv, err
}

func (r *AIConversationRepositoryImpl) Update(conv *model.AIConversation) error {
	return r.db.Save(conv).Error
}

// Delete 删除会话及其消息。
//
// 必须先按 id+user_id 删会话，确认真的删掉一行后才清消息：
// 原实现无条件按 conversation_id 删消息，任何登录用户拿别人的会话 ID
// 就能清空对方聊天记录，而对方会话行因 user_id 不匹配反而保留。
// 非本人或不存在的 ID 静默成功（幂等），不额外泄露会话是否存在。
func (r *AIConversationRepositoryImpl) Delete(id, userID uint64) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		res := tx.Where("id = ? AND user_id = ?", id, userID).Delete(&model.AIConversation{})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return nil // 不属于当前用户：一条消息都不许删
		}
		return tx.Where("conversation_id = ?", id).Delete(&model.AIMessage{}).Error
	})
}

// ---- AI Message ----

type AIMessageRepository interface {
	Create(msg *model.AIMessage) error
	FindByConversation(conversationID uint64) ([]model.AIMessage, error)
}

type AIMessageRepositoryImpl struct {
	db *gorm.DB
}

func NewAIMessageRepository(db *gorm.DB) AIMessageRepository {
	return &AIMessageRepositoryImpl{db: db}
}

func (r *AIMessageRepositoryImpl) Create(msg *model.AIMessage) error {
	return r.db.Create(msg).Error
}

func (r *AIMessageRepositoryImpl) FindByConversation(conversationID uint64) ([]model.AIMessage, error) {
	var list []model.AIMessage
	err := r.db.Where("conversation_id = ?", conversationID).Order("id ASC").Find(&list).Error
	return list, err
}

// ---- Membership ----
