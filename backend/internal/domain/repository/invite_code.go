package repository

import (
	"time"

	"software-exam/internal/domain/model"

	"gorm.io/gorm"
)

type InviteCodeRepository interface {
	BatchCreate(codes []model.InviteCode) error
	FindByCode(code string) (*model.InviteCode, error)
	MarkUsed(code string, userID uint64) error
	List(page, pageSize int) ([]model.InviteCode, int64, error)
	Delete(id uint64) error
}

type InviteCodeRepositoryImpl struct {
	db *gorm.DB
}

func NewInviteCodeRepository(db *gorm.DB) InviteCodeRepository {
	return &InviteCodeRepositoryImpl{db: db}
}

func (r *InviteCodeRepositoryImpl) BatchCreate(codes []model.InviteCode) error {
	return r.db.Create(&codes).Error
}

func (r *InviteCodeRepositoryImpl) FindByCode(code string) (*model.InviteCode, error) {
	var c model.InviteCode
	err := r.db.Where("code = ?", code).First(&c).Error
	return &c, err
}

func (r *InviteCodeRepositoryImpl) MarkUsed(code string, userID uint64) error {
	now := time.Now()
	return r.db.Model(&model.InviteCode{}).Where("code = ? AND status = 0", code).
		Updates(map[string]interface{}{"status": 1, "used_by": userID, "used_at": now}).Error
}

func (r *InviteCodeRepositoryImpl) List(page, pageSize int) ([]model.InviteCode, int64, error) {
	var list []model.InviteCode
	var total int64
	r.db.Model(&model.InviteCode{}).Count(&total)
	err := r.db.Order("created_at DESC").
		Offset((page - 1) * pageSize).Limit(pageSize).
		Find(&list).Error
	return list, total, err
}

func (r *InviteCodeRepositoryImpl) Delete(id uint64) error {
	return r.db.Where("id = ? AND status = 0", id).Delete(&model.InviteCode{}).Error
}

// ---- CardKey（会员兑换卡密，结构仿 InviteCode）----

type CardKeyListReq struct {
	Page     int
	PageSize int
	Keyword  string // 模糊匹配卡密 code / 备注 note
	Plan     string // 精确匹配套餐；为空时不过滤
	Status   int8   // -1=不过滤；0=未使用；1=已使用
}
