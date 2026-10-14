package repository

import (
	"time"

	"software-exam/internal/domain/model"

	"gorm.io/gorm"
)

type NotificationRepository interface {
	Create(n *model.Notification) error
	ListByUser(userID uint64, page, pageSize int, onlyUnread bool) ([]model.Notification, int64, error)
	CountUnread(userID uint64) (int64, error)
	MarkRead(id, userID uint64) error
	MarkAllRead(userID uint64) error
}

type NotificationRepositoryImpl struct {
	db *gorm.DB
}

func NewNotificationRepository(db *gorm.DB) NotificationRepository {
	return &NotificationRepositoryImpl{db: db}
}

func (r *NotificationRepositoryImpl) Create(n *model.Notification) error {
	return r.db.Create(n).Error
}

func (r *NotificationRepositoryImpl) ListByUser(userID uint64, page, pageSize int, onlyUnread bool) ([]model.Notification, int64, error) {
	var list []model.Notification
	var total int64

	base := r.db.Model(&model.Notification{}).Where("user_id = ?", userID)
	if onlyUnread {
		base = base.Where("read_at IS NULL")
	}
	base.Count(&total)

	err := base.Order("id DESC").
		Offset((page - 1) * pageSize).Limit(pageSize).
		Find(&list).Error
	return list, total, err
}

func (r *NotificationRepositoryImpl) CountUnread(userID uint64) (int64, error) {
	var count int64
	err := r.db.Model(&model.Notification{}).
		Where("user_id = ? AND read_at IS NULL", userID).Count(&count).Error
	return count, err
}

// MarkRead 的 user_id 条件不是冗余：少了它任何人都能标记别人的通知已读。
func (r *NotificationRepositoryImpl) MarkRead(id, userID uint64) error {
	return r.db.Model(&model.Notification{}).
		Where("id = ? AND user_id = ? AND read_at IS NULL", id, userID).
		Update("read_at", time.Now()).Error
}

func (r *NotificationRepositoryImpl) MarkAllRead(userID uint64) error {
	return r.db.Model(&model.Notification{}).
		Where("user_id = ? AND read_at IS NULL", userID).
		Update("read_at", time.Now()).Error
}

// ---- QuestionReport ----

// QuestionReportView 用于管理端列表，JOIN 了用户名和题目信息，
// 让管理员不看原始题目也能定位：标题/年份/题型 + 题干摘要。
type QuestionReportView struct {
	ID              uint64    `json:"id" gorm:"column:id"`
	QuestionID      uint64    `json:"question_id" gorm:"column:question_id"`
	UserID          uint64    `json:"user_id" gorm:"column:user_id"`
	Username        string    `json:"username" gorm:"column:username"`
	ReportType      int8      `json:"report_type" gorm:"column:report_type"`
	Description     string    `json:"description" gorm:"column:description"`
	Status          int8      `json:"status" gorm:"column:status"`
	AdminNote       string    `json:"admin_note" gorm:"column:admin_note"`
	CreatedAt       time.Time `json:"created_at" gorm:"column:created_at"`
	QuestionTitle   string    `json:"question_title" gorm:"column:question_title"`
	QuestionYear    int16     `json:"question_year" gorm:"column:question_year"`
	QuestionType    int8      `json:"question_type" gorm:"column:question_type"`
	QuestionContent string    `json:"question_content" gorm:"column:question_content"`
	QuestionPreview string    `json:"question_preview" gorm:"column:question_preview"` // 兼容旧前端
	// 科目名经 questions.subject_id 关联出来；题目缺失时为 NULL → 零值空串
	SubjectName     string    `json:"subject_name" gorm:"column:subject_name"`
}
