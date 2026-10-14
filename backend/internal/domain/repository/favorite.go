package repository

import (
	"software-exam/internal/domain/model"

	"gorm.io/gorm"
)

type FavoriteRepository interface {
	Create(fav *model.Favorite) error
	Delete(userID uint64, targetType int8, targetID uint64) error
	FindByUserAndTarget(userID uint64, targetType int8, targetID uint64) (*model.Favorite, error)
	FindByUserID(userID uint64, targetType int8, page, pageSize int) ([]model.Favorite, int64, error)
}

type FavoriteRepositoryImpl struct {
	db *gorm.DB
}

func NewFavoriteRepository(db *gorm.DB) FavoriteRepository {
	return &FavoriteRepositoryImpl{db: db}
}

func (r *FavoriteRepositoryImpl) Create(fav *model.Favorite) error {
	return r.db.Create(fav).Error
}

func (r *FavoriteRepositoryImpl) Delete(userID uint64, targetType int8, targetID uint64) error {
	return r.db.Where("user_id = ? AND target_type = ? AND target_id = ?", userID, targetType, targetID).Delete(&model.Favorite{}).Error
}

func (r *FavoriteRepositoryImpl) FindByUserAndTarget(userID uint64, targetType int8, targetID uint64) (*model.Favorite, error) {
	var fav model.Favorite
	err := r.db.Where("user_id = ? AND target_type = ? AND target_id = ?", userID, targetType, targetID).First(&fav).Error
	return &fav, err
}

func (r *FavoriteRepositoryImpl) FindByUserID(userID uint64, targetType int8, page, pageSize int) ([]model.Favorite, int64, error) {
	var favs []model.Favorite
	var total int64
	query := r.db.Where("user_id = ? AND target_type = ?", userID, targetType)
	query.Model(&model.Favorite{}).Count(&total)
	// created_at 相同的数据翻页需要 id tie-breaker，否则会重复/丢行
	err := query.Order("created_at DESC, id DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&favs).Error
	return favs, total, err
}
