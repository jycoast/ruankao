package repository

import (
	"software-exam/internal/domain/model"

	"gorm.io/gorm"
)

type AnnouncementRepository interface {
	Create(a *model.Announcement) error
	FindByID(id uint64) (*model.Announcement, error)
	Update(a *model.Announcement) error
	Delete(id uint64) error
	ListAll(page, pageSize int) ([]model.Announcement, int64, error)
	ListUnread(userID uint64) ([]model.Announcement, error)
	MarkRead(userID, announcementID uint64) error
}

type AnnouncementRepositoryImpl struct {
	db *gorm.DB
}

func NewAnnouncementRepository(db *gorm.DB) AnnouncementRepository {
	return &AnnouncementRepositoryImpl{db: db}
}

func (r *AnnouncementRepositoryImpl) Create(a *model.Announcement) error {
	return r.db.Create(a).Error
}

func (r *AnnouncementRepositoryImpl) FindByID(id uint64) (*model.Announcement, error) {
	var a model.Announcement
	err := r.db.First(&a, id).Error
	return &a, err
}

func (r *AnnouncementRepositoryImpl) Update(a *model.Announcement) error {
	return r.db.Save(a).Error
}

func (r *AnnouncementRepositoryImpl) Delete(id uint64) error {
	return r.db.Delete(&model.Announcement{}, id).Error
}

func (r *AnnouncementRepositoryImpl) ListAll(page, pageSize int) ([]model.Announcement, int64, error) {
	var list []model.Announcement
	var total int64
	r.db.Model(&model.Announcement{}).Count(&total)
	err := r.db.Offset((page - 1) * pageSize).Limit(pageSize).Order("created_at DESC").Find(&list).Error
	return list, total, err
}

func (r *AnnouncementRepositoryImpl) ListUnread(userID uint64) ([]model.Announcement, error) {
	var list []model.Announcement
	err := r.db.
		Where("status = 1").
		Where("id NOT IN (?)", r.db.Model(&model.AnnouncementRead{}).
			Select("announcement_id").Where("user_id = ?", userID)).
		Order("created_at DESC").
		Find(&list).Error
	return list, err
}

func (r *AnnouncementRepositoryImpl) MarkRead(userID, announcementID uint64) error {
	// 唯一键 (user_id, announcement_id) 保证幂等：已存在则不重复插入
	return r.db.Where("user_id = ? AND announcement_id = ?", userID, announcementID).
		FirstOrCreate(&model.AnnouncementRead{UserID: userID, AnnouncementID: announcementID}).Error
}

// ---- Notification ----
