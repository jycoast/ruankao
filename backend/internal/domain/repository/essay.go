package repository

import (
	"software-exam/internal/domain/model"

	"gorm.io/gorm"
)

type EssaySubmissionRepository interface {
	Create(s *model.EssaySubmission) error
	FindByID(id uint64) (*model.EssaySubmission, error)
	Update(s *model.EssaySubmission) error
	ListByUserID(userID uint64, page, pageSize int) ([]model.EssaySubmission, int64, error)
	ListAll(page, pageSize, status int) ([]EssaySubmissionView, int64, error)
	CountPending() (int64, error)
}

type EssaySubmissionRepositoryImpl struct {
	db *gorm.DB
}

func NewEssaySubmissionRepository(db *gorm.DB) EssaySubmissionRepository {
	return &EssaySubmissionRepositoryImpl{db: db}
}

func (r *EssaySubmissionRepositoryImpl) Create(s *model.EssaySubmission) error {
	return r.db.Create(s).Error
}

func (r *EssaySubmissionRepositoryImpl) FindByID(id uint64) (*model.EssaySubmission, error) {
	var s model.EssaySubmission
	err := r.db.First(&s, id).Error
	return &s, err
}

func (r *EssaySubmissionRepositoryImpl) Update(s *model.EssaySubmission) error {
	return r.db.Save(s).Error
}

func (r *EssaySubmissionRepositoryImpl) ListByUserID(userID uint64, page, pageSize int) ([]model.EssaySubmission, int64, error) {
	var list []model.EssaySubmission
	var total int64
	r.db.Model(&model.EssaySubmission{}).Where("user_id = ?", userID).Count(&total)
	err := r.db.Where("user_id = ?", userID).
		Order("created_at DESC").
		Offset((page - 1) * pageSize).Limit(pageSize).
		Find(&list).Error
	return list, total, err
}

func (r *EssaySubmissionRepositoryImpl) ListAll(page, pageSize, status int) ([]EssaySubmissionView, int64, error) {
	var list []EssaySubmissionView
	var total int64

	base := r.db.Table("essay_submissions es").
		Select("es.id, es.user_id, es.title, es.category, es.year, es.score, es.review_status, es.reviewer_note, es.article_id, es.created_at, u.username").
		Joins("LEFT JOIN users u ON u.id = es.user_id")

	if status >= 0 {
		base = base.Where("es.review_status = ?", status)
	}

	base.Count(&total)
	err := base.Order("es.created_at DESC").
		Offset((page - 1) * pageSize).Limit(pageSize).
		Scan(&list).Error

	return list, total, err
}

func (r *EssaySubmissionRepositoryImpl) CountPending() (int64, error) {
	var count int64
	err := r.db.Model(&model.EssaySubmission{}).Where("review_status = 0").Count(&count).Error
	return count, err
}

// ---- InviteCode ----
