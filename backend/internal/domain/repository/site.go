package repository

import (
	"errors"

	"software-exam/internal/domain/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// siteSettingsID 单行表的固定主键。与 ai_configs 一样，全表就这一行。
const siteSettingsID = 1

// SiteSettingsRepository 站点级开关的读写口。
// 有意不暴露「整行」——调用方只需要知道维护开没开，拿到 *model.SiteSettings
// 会让上层有机会去改 UpdatedAt 之类的字段。
type SiteSettingsRepository interface {
	MaintenanceEnabled() (bool, error)
	UpdateMaintenance(enabled bool) error
}

type siteSettingsRepository struct {
	db *gorm.DB
}

func NewSiteSettingsRepository(db *gorm.DB) SiteSettingsRepository {
	return &siteSettingsRepository{db: db}
}

// MaintenanceEnabled 读维护开关。
// 行不存在（迁移没跑）时返回 false 而不是报错：把「查不到」当维护中会把整站锁死，
// 而这时候连后台都进不去。库连接层面的错误照常往上抛，由 service 决定怎么兜。
func (r *siteSettingsRepository) MaintenanceEnabled() (bool, error) {
	var row model.SiteSettings
	err := r.db.Where("id = ?", siteSettingsID).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return row.MaintenanceEnabled == 1, nil
}

// UpdateMaintenance 写维护开关。
// 用 upsert 而非裸 Update：裸 Update 在行缺失时 RowsAffected=0 且不报错，
// 开关会「保存成功但没生效」，排查起来很费劲。ON CONFLICT 让它在任何情况下都落地。
func (r *siteSettingsRepository) UpdateMaintenance(enabled bool) error {
	v := int8(0)
	if enabled {
		v = 1
	}
	return r.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "id"}},
		DoUpdates: clause.AssignmentColumns([]string{"maintenance_enabled", "updated_at"}),
	}).Create(&model.SiteSettings{ID: siteSettingsID, MaintenanceEnabled: v}).Error
}
