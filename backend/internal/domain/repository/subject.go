package repository

import (
	"software-exam/internal/domain/model"

	"gorm.io/gorm"
)

type SubjectRepository interface {
	List() ([]model.Subject, error)
	FindByID(id uint64) (*model.Subject, error)
}

type SubjectRepositoryImpl struct {
	db *gorm.DB
}

func NewSubjectRepository(db *gorm.DB) SubjectRepository {
	return &SubjectRepositoryImpl{db: db}
}

func (r *SubjectRepositoryImpl) List() ([]model.Subject, error) {
	var subjects []model.Subject
	err := r.db.Where("status = 1").Order("sort_order ASC").Find(&subjects).Error
	return subjects, err
}

func (r *SubjectRepositoryImpl) FindByID(id uint64) (*model.Subject, error) {
	var s model.Subject
	err := r.db.First(&s, id).Error
	return &s, err
}
