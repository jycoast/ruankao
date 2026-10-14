package repository

import (
	"time"

	"software-exam/internal/domain/model"

	"gorm.io/gorm"
)

type PasswordResetRepository interface {
	Create(p *model.PasswordReset) error
	FindByID(id uint64) (*model.PasswordReset, error)
	ListAll(page, pageSize, status int) ([]PasswordResetView, int64, error)
	UpdateStatus(id uint64, status int8, adminNote string) error
	CountPending() (int64, error)
	CountPendingByUser(userID uint64) (int64, error)
}

type PasswordResetRepositoryImpl struct {
	db *gorm.DB
}

func NewPasswordResetRepository(db *gorm.DB) PasswordResetRepository {
	return &PasswordResetRepositoryImpl{db: db}
}

func (r *PasswordResetRepositoryImpl) Create(p *model.PasswordReset) error {
	return r.db.Create(p).Error
}

func (r *PasswordResetRepositoryImpl) FindByID(id uint64) (*model.PasswordReset, error) {
	var p model.PasswordReset
	if err := r.db.First(&p, id).Error; err != nil {
		return nil, err
	}
	return &p, nil
}

func (r *PasswordResetRepositoryImpl) ListAll(page, pageSize, status int) ([]PasswordResetView, int64, error) {
	var list []PasswordResetView
	var total int64

	base := r.db.Table("password_resets pr").
		Select("pr.id, pr.user_id, pr.account, pr.contact, pr.reason, pr.client_ip, pr.status, pr.admin_note, pr.created_at, u.username, u.email").
		Joins("LEFT JOIN users u ON u.id = pr.user_id")

	if status >= 0 {
		base = base.Where("pr.status = ?", status)
	}

	base.Count(&total)
	err := base.Order("pr.created_at DESC").
		Offset((page - 1) * pageSize).Limit(pageSize).
		Scan(&list).Error

	return list, total, err
}

func (r *PasswordResetRepositoryImpl) UpdateStatus(id uint64, status int8, adminNote string) error {
	return r.db.Model(&model.PasswordReset{}).Where("id = ?", id).
		Updates(map[string]interface{}{"status": status, "admin_note": adminNote, "updated_at": time.Now()}).Error
}

func (r *PasswordResetRepositoryImpl) CountPending() (int64, error) {
	var count int64
	err := r.db.Model(&model.PasswordReset{}).Where("status = 0").Count(&count).Error
	return count, err
}

// CountPendingByUser 同一账号已堆积的待处理申请数。
// 限流按 IP 拦不住换 IP 重刷，这里再挡一层，避免一个账号刷出几百条把后台淹了。
func (r *PasswordResetRepositoryImpl) CountPendingByUser(userID uint64) (int64, error) {
	var count int64
	err := r.db.Model(&model.PasswordReset{}).
		Where("user_id = ? AND status = 0", userID).Count(&count).Error
	return count, err
}

// ---- EssaySubmission ----

type EssaySubmissionView struct {
	ID           uint64    `json:"id" gorm:"column:id"`
	UserID       uint64    `json:"user_id" gorm:"column:user_id"`
	Username     string    `json:"username" gorm:"column:username"`
	Title        string    `json:"title" gorm:"column:title"`
	Category     string    `json:"category" gorm:"column:category"`
	Year         int16     `json:"year" gorm:"column:year"`
	Score        float32   `json:"score" gorm:"column:score"`
	ReviewStatus int8      `json:"review_status" gorm:"column:review_status"`
	ReviewerNote string    `json:"reviewer_note" gorm:"column:reviewer_note"`
	ArticleID    uint64    `json:"article_id" gorm:"column:article_id"`
	CreatedAt    time.Time `json:"created_at" gorm:"column:created_at"`
}
