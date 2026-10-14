package repository

import (
	"errors"

	"software-exam/internal/domain/model"

	"gorm.io/gorm"
)

type ExamRecordRepository interface {
	Create(record *model.ExamRecord) error
	FindByID(id uint64) (*model.ExamRecord, error)
	FindInProgress(userID, paperID uint64) (*model.ExamRecord, error)
	Update(record *model.ExamRecord) error
	FinishOnce(record *model.ExamRecord) (bool, error)
	FindByUserID(userID uint64, page, pageSize int) ([]model.ExamRecord, int64, error)
}

type ExamRecordRepositoryImpl struct {
	db *gorm.DB
}

func NewExamRecordRepository(db *gorm.DB) ExamRecordRepository {
	return &ExamRecordRepositoryImpl{db: db}
}

func (r *ExamRecordRepositoryImpl) Create(record *model.ExamRecord) error {
	return r.db.Create(record).Error
}

func (r *ExamRecordRepositoryImpl) FindByID(id uint64) (*model.ExamRecord, error) {
	var er model.ExamRecord
	err := r.db.First(&er, id).Error
	return &er, err
}

// FindInProgress 取该用户这套卷子上尚未交卷的记录，没有则返回 (nil, nil)。
// 取最近一条：历史数据里可能已经堆了多条僵尸记录，接着最新的那场考。
func (r *ExamRecordRepositoryImpl) FindInProgress(userID, paperID uint64) (*model.ExamRecord, error) {
	var er model.ExamRecord
	err := r.db.Where("user_id = ? AND paper_id = ? AND status = 0", userID, paperID).
		Order("id DESC").First(&er).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &er, nil
}

func (r *ExamRecordRepositoryImpl) Update(record *model.ExamRecord) error {
	return r.db.Save(record).Error
}

// FinishOnce 仅当记录仍处于「进行中」时写入成绩并置为已完成。
// 返回 false 表示这场考试已交过卷，调用方应拒绝重复交卷。
func (r *ExamRecordRepositoryImpl) FinishOnce(record *model.ExamRecord) (bool, error) {
	res := r.db.Model(&model.ExamRecord{}).
		Where("id = ? AND status = 0", record.ID).
		Updates(map[string]interface{}{
			"score":           record.Score,
			"total_questions": record.TotalQuestions,
			"correct_count":   record.CorrectCount,
			"time_spent":      record.TimeSpent,
			"finished_at":     record.FinishedAt,
			"status":          1,
		})
	if res.Error != nil {
		return false, res.Error
	}
	return res.RowsAffected > 0, nil
}

func (r *ExamRecordRepositoryImpl) FindByUserID(userID uint64, page, pageSize int) ([]model.ExamRecord, int64, error) {
	var records []model.ExamRecord
	var total int64
	query := r.db.Where("user_id = ?", userID)
	query.Model(&model.ExamRecord{}).Count(&total)
	// created_at 相同的数据翻页需要 id tie-breaker，否则会重复/丢行
	err := query.Order("created_at DESC, id DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&records).Error
	return records, total, err
}
