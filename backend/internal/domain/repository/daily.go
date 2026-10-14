package repository

import (
	"software-exam/internal/domain/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type DailyCheckinRepository interface {
	FindByUserAndDate(userID uint64, date string) (*model.DailyCheckin, error)
	Upsert(c *model.DailyCheckin) error
	ListByUserInRange(userID uint64, startDate, endDate string) ([]model.DailyCheckin, error)
}

type DailyCheckinRepositoryImpl struct {
	db *gorm.DB
}

func NewDailyCheckinRepository(db *gorm.DB) DailyCheckinRepository {
	return &DailyCheckinRepositoryImpl{db: db}
}

func (r *DailyCheckinRepositoryImpl) FindByUserAndDate(userID uint64, date string) (*model.DailyCheckin, error) {
	var c model.DailyCheckin
	err := r.db.Where("user_id = ? AND checkin_date = ?", userID, date).First(&c).Error
	if err != nil {
		return nil, err
	}
	return &c, nil
}

// Upsert 幂等写入当日签到：存在则更新答题/正确数，否则创建。
func (r *DailyCheckinRepositoryImpl) Upsert(c *model.DailyCheckin) error {
	existing := &model.DailyCheckin{}
	err := r.db.Where("user_id = ? AND checkin_date = ?", c.UserID, c.CheckinDate).First(existing).Error
	if err != nil {
		return r.db.Create(c).Error
	}
	existing.QuestionCount = c.QuestionCount
	existing.CorrectCount = c.CorrectCount
	return r.db.Save(existing).Error
}

func (r *DailyCheckinRepositoryImpl) ListByUserInRange(userID uint64, startDate, endDate string) ([]model.DailyCheckin, error) {
	var list []model.DailyCheckin
	err := r.db.Where("user_id = ? AND checkin_date >= ? AND checkin_date <= ?", userID, startDate, endDate).
		Order("checkin_date ASC").Find(&list).Error
	return list, err
}

// ---- DailySession（每日一练断点草稿）----

type DailySessionRepository interface {
	FindOne(userID uint64, date string, subjectID uint64) (*model.DailySession, error)
	Save(session *model.DailySession) error
	Delete(userID uint64, date string, subjectID uint64) error
}

type DailySessionRepositoryImpl struct {
	db *gorm.DB
}

func NewDailySessionRepository(db *gorm.DB) DailySessionRepository {
	return &DailySessionRepositoryImpl{db: db}
}

func (r *DailySessionRepositoryImpl) scope(userID uint64, date string, subjectID uint64) *gorm.DB {
	return r.db.Where("user_id = ? AND date = ? AND subject_id = ?", userID, date, subjectID)
}

func (r *DailySessionRepositoryImpl) FindOne(userID uint64, date string, subjectID uint64) (*model.DailySession, error) {
	var s model.DailySession
	if err := r.scope(userID, date, subjectID).First(&s).Error; err != nil {
		return nil, err
	}
	return &s, nil
}

func (r *DailySessionRepositoryImpl) Save(session *model.DailySession) error {
	return r.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "user_id"}, {Name: "date"}, {Name: "subject_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"data", "updated_at"}),
	}).Create(session).Error
}

func (r *DailySessionRepositoryImpl) Delete(userID uint64, date string, subjectID uint64) error {
	return r.scope(userID, date, subjectID).Delete(&model.DailySession{}).Error
}

// ---- QuestionNote ----
