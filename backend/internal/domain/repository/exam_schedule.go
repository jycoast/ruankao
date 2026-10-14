package repository

import (
	"software-exam/internal/domain/model"

	"gorm.io/gorm"
)

type ExamScheduleRepository interface {
	List() ([]model.ExamSchedule, error)
	FindByID(id uint64) (*model.ExamSchedule, error)
	Create(s *model.ExamSchedule) error
	Update(s *model.ExamSchedule) error
	Delete(id uint64) error
}

type ExamScheduleRepositoryImpl struct {
	db *gorm.DB
}

func NewExamScheduleRepository(db *gorm.DB) ExamScheduleRepository {
	return &ExamScheduleRepositoryImpl{db: db}
}

func (r *ExamScheduleRepositoryImpl) List() ([]model.ExamSchedule, error) {
	var list []model.ExamSchedule
	err := r.db.Order("exam_date ASC").Find(&list).Error
	return list, err
}

func (r *ExamScheduleRepositoryImpl) FindByID(id uint64) (*model.ExamSchedule, error) {
	var s model.ExamSchedule
	err := r.db.First(&s, id).Error
	return &s, err
}

func (r *ExamScheduleRepositoryImpl) Create(s *model.ExamSchedule) error {
	return r.db.Create(s).Error
}

func (r *ExamScheduleRepositoryImpl) Update(s *model.ExamSchedule) error {
	// 不回写 created_at：种子数据该列为 NULL，Save 全量写回会把零值时间
	// 发成 '0000-00-00'，被 MySQL 8 严格模式拒绝（Error 1292）
	return r.db.Omit("created_at").Save(s).Error
}

func (r *ExamScheduleRepositoryImpl) Delete(id uint64) error {
	return r.db.Delete(&model.ExamSchedule{}, id).Error
}
