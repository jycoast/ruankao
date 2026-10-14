package repository

import (
	"time"

	"software-exam/internal/domain/model"

	"gorm.io/gorm"
)

type CardKeyRepository interface {
	BatchCreate(codes []model.CardKey) error
	FindByCode(code string) (*model.CardKey, error)
	MarkUsed(code string, userID uint64) error
	List(req CardKeyListReq) ([]model.CardKey, int64, error)
	Delete(id uint64) error
}

type CardKeyRepositoryImpl struct {
	db *gorm.DB
}

func NewCardKeyRepository(db *gorm.DB) CardKeyRepository {
	return &CardKeyRepositoryImpl{db: db}
}

func (r *CardKeyRepositoryImpl) BatchCreate(codes []model.CardKey) error {
	return r.db.Create(&codes).Error
}

func (r *CardKeyRepositoryImpl) FindByCode(code string) (*model.CardKey, error) {
	var c model.CardKey
	err := r.db.Where("code = ?", code).First(&c).Error
	return &c, err
}

// MarkUsed 原子占用：仅当 status=0 时置为已用，防止一码多兑。返回受影响行数为 0 视为已被占用。
func (r *CardKeyRepositoryImpl) MarkUsed(code string, userID uint64) error {
	now := time.Now()
	res := r.db.Model(&model.CardKey{}).Where("code = ? AND status = 0", code).
		Updates(map[string]interface{}{"status": 1, "used_by": userID, "used_at": now})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (r *CardKeyRepositoryImpl) List(req CardKeyListReq) ([]model.CardKey, int64, error) {
	var list []model.CardKey
	var total int64
	query := r.db.Model(&model.CardKey{})
	if req.Plan != "" {
		query = query.Where("plan = ?", req.Plan)
	}
	if req.Status != -1 {
		query = query.Where("status = ?", req.Status)
	}
	if req.Keyword != "" {
		like := "%" + req.Keyword + "%"
		query = query.Where("code LIKE ? OR note LIKE ?", like, like)
	}
	query.Count(&total)
	err := query.Order("created_at DESC").
		Offset((req.Page - 1) * req.PageSize).Limit(req.PageSize).
		Find(&list).Error
	return list, total, err
}

func (r *CardKeyRepositoryImpl) Delete(id uint64) error {
	return r.db.Where("id = ? AND status = 0", id).Delete(&model.CardKey{}).Error
}

// ---- DailyCheckin（每日一练签到）----
