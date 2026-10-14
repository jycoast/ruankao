package repository

import (
	"software-exam/internal/domain/model"

	"gorm.io/gorm"
)

type WrongQuestionRepository interface {
	Create(wq *model.WrongQuestion) error
	FindByUserAndQuestion(userID, questionID uint64) (*model.WrongQuestion, error)
	Update(wq *model.WrongQuestion) error
	Delete(userID, questionID uint64) error
	FindByUserID(userID uint64, status int8, page, pageSize int) ([]model.WrongQuestion, int64, error)
	FindDueReview(userID uint64, page, pageSize int) ([]model.WrongQuestion, int64, error)
}

type WrongQuestionRepositoryImpl struct {
	db *gorm.DB
}

func NewWrongQuestionRepository(db *gorm.DB) WrongQuestionRepository {
	return &WrongQuestionRepositoryImpl{db: db}
}

func (r *WrongQuestionRepositoryImpl) Create(wq *model.WrongQuestion) error {
	return r.db.Create(wq).Error
}

func (r *WrongQuestionRepositoryImpl) FindByUserAndQuestion(userID, questionID uint64) (*model.WrongQuestion, error) {
	var wq model.WrongQuestion
	err := r.db.Where("user_id = ? AND question_id = ?", userID, questionID).First(&wq).Error
	return &wq, err
}

func (r *WrongQuestionRepositoryImpl) Update(wq *model.WrongQuestion) error {
	return r.db.Save(wq).Error
}

func (r *WrongQuestionRepositoryImpl) Delete(userID, questionID uint64) error {
	return r.db.Where("user_id = ? AND question_id = ?", userID, questionID).Delete(&model.WrongQuestion{}).Error
}

func (r *WrongQuestionRepositoryImpl) FindByUserID(userID uint64, status int8, page, pageSize int) ([]model.WrongQuestion, int64, error) {
	var wqs []model.WrongQuestion
	var total int64
	query := r.db.Where("user_id = ?", userID)
	if status > 0 {
		query = query.Where("status = ?", status)
	}
	query.Model(&model.WrongQuestion{}).Count(&total)
	// 排序带 id DESC tie-breaker：created_at 相同的数据翻页才不会重复/丢行
	err := query.Preload("Question").Preload("Question.Options", func(db *gorm.DB) *gorm.DB { return db.Order("id ASC") }).
		Order("created_at DESC, id DESC").
		Offset((page - 1) * pageSize).Limit(pageSize).Find(&wqs).Error
	return wqs, total, err
}

func (r *WrongQuestionRepositoryImpl) FindDueReview(userID uint64, page, pageSize int) ([]model.WrongQuestion, int64, error) {
	var wqs []model.WrongQuestion
	var total int64
	query := r.db.Where("user_id = ? AND status = 1 AND next_review_time <= NOW()", userID)
	query.Model(&model.WrongQuestion{}).Count(&total)
	// 原实现无排序——分页后翻页会乱序/重复；到期最早的最先复习
	err := query.Preload("Question").Preload("Question.Options", func(db *gorm.DB) *gorm.DB { return db.Order("id ASC") }).
		Order("next_review_time ASC, id ASC").
		Offset((page - 1) * pageSize).Limit(pageSize).Find(&wqs).Error
	return wqs, total, err
}
