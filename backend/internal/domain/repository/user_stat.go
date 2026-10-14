package repository

import (
	"software-exam/internal/domain/model"

	"gorm.io/gorm"
)

type UserProfileRepository interface {
	Create(profile *model.UserProfile) error
	FindByUserID(userID uint64) (*model.UserProfile, error)
	Update(profile *model.UserProfile) error
}

type UserProfileRepositoryImpl struct {
	db *gorm.DB
}

func NewUserProfileRepository(db *gorm.DB) UserProfileRepository {
	return &UserProfileRepositoryImpl{db: db}
}

func (r *UserProfileRepositoryImpl) Create(profile *model.UserProfile) error {
	return r.db.Create(profile).Error
}

func (r *UserProfileRepositoryImpl) FindByUserID(userID uint64) (*model.UserProfile, error) {
	var profile model.UserProfile
	err := r.db.Where("user_id = ?", userID).First(&profile).Error
	return &profile, err
}

func (r *UserProfileRepositoryImpl) Update(profile *model.UserProfile) error {
	return r.db.Save(profile).Error
}

// ---- User Subject Stats 按科目维度累加 ----

type UserSubjectStatRepository interface {
	FindOne(userID, subjectID uint64) (*model.UserSubjectStat, error)
	Create(stat *model.UserSubjectStat) error
	Save(stat *model.UserSubjectStat) error
}

type UserSubjectStatRepositoryImpl struct {
	db *gorm.DB
}

func NewUserSubjectStatRepository(db *gorm.DB) UserSubjectStatRepository {
	return &UserSubjectStatRepositoryImpl{db: db}
}

// FindOne 取该用户该科目的累加行；不存在返回 ErrRecordNotFound。
func (r *UserSubjectStatRepositoryImpl) FindOne(userID, subjectID uint64) (*model.UserSubjectStat, error) {
	var stat model.UserSubjectStat
	err := r.db.Where("user_id = ? AND subject_id = ?", userID, subjectID).First(&stat).Error
	return &stat, err
}

func (r *UserSubjectStatRepositoryImpl) Create(stat *model.UserSubjectStat) error {
	return r.db.Create(stat).Error
}

// Save 写回累加后的整行（存在唯一键索引主键已存在用 Update）。
func (r *UserSubjectStatRepositoryImpl) Save(stat *model.UserSubjectStat) error {
	return r.db.Save(stat).Error
}
