package repository

import (
	"time"

	"software-exam/internal/domain/model"

	"gorm.io/gorm"
)

type MembershipRepository interface {
	FindByUserID(userID uint64) (*model.Membership, error)
	FindByUserIDs(ids []uint64) ([]model.Membership, error)
	ExistsByUserID(userID uint64) (bool, error)
	Upsert(m *model.Membership) error
}

type MembershipRepositoryImpl struct {
	db *gorm.DB
}

func NewMembershipRepository(db *gorm.DB) MembershipRepository {
	return &MembershipRepositoryImpl{db: db}
}

func (r *MembershipRepositoryImpl) FindByUserID(userID uint64) (*model.Membership, error) {
	var m model.Membership
	err := r.db.Where("user_id = ?", userID).First(&m).Error
	if err != nil {
		return nil, err
	}
	return &m, nil
}

// FindByUserIDs 批量查询多个用户的会员记录（管理后台列表用，避免逐用户点查）。
func (r *MembershipRepositoryImpl) FindByUserIDs(ids []uint64) ([]model.Membership, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	var list []model.Membership
	err := r.db.Where("user_id IN ?", ids).Find(&list).Error
	return list, err
}

func (r *MembershipRepositoryImpl) Upsert(m *model.Membership) error {
	var existing model.Membership
	err := r.db.Where("user_id = ?", m.UserID).First(&existing).Error
	if err != nil {
		return r.db.Create(m).Error
	}
	existing.Plan = m.Plan
	existing.ExpireAt = m.ExpireAt
	return r.db.Save(&existing).Error
}

// ExistsByUserID 判断该用户是否已有会员记录（含已过期）。首次登录赠送用它做幂等：
// 只要 memberships 里存在过该 user_id，就不再重复发放。
func (r *MembershipRepositoryImpl) ExistsByUserID(userID uint64) (bool, error) {
	var count int64
	err := r.db.Model(&model.Membership{}).Where("user_id = ?", userID).Count(&count).Error
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// ---- MembershipPlan ----

type MembershipPlanRepository interface {
	List() ([]model.MembershipPlan, error)
	FindByKey(key string) (*model.MembershipPlan, error)
	Update(p *model.MembershipPlan) error
}

type MembershipPlanRepositoryImpl struct {
	db *gorm.DB
}

func NewMembershipPlanRepository(db *gorm.DB) MembershipPlanRepository {
	return &MembershipPlanRepositoryImpl{db: db}
}

func (r *MembershipPlanRepositoryImpl) List() ([]model.MembershipPlan, error) {
	var list []model.MembershipPlan
	err := r.db.Order("sort_order ASC, id ASC").Find(&list).Error
	return list, err
}

func (r *MembershipPlanRepositoryImpl) FindByKey(key string) (*model.MembershipPlan, error) {
	var p model.MembershipPlan
	if err := r.db.Where("plan_key = ?", key).First(&p).Error; err != nil {
		return nil, err
	}
	return &p, nil
}

// Update 只改价格与展示字段。plan_key/days 不在此变更：
// days 决定已售卡密的时长，plan_key 是订单与卡密的外键，改了会让存量记录对不上套餐。
func (r *MembershipPlanRepositoryImpl) Update(p *model.MembershipPlan) error {
	return r.db.Model(&model.MembershipPlan{}).
		Where("plan_key = ?", p.PlanKey).
		Updates(map[string]interface{}{
			"name":       p.Name,
			"amount":     p.Amount,
			"ldc_amount": p.LDCAmount,
			"sort_order": p.SortOrder,
			"enabled":    p.Enabled,
		}).Error
}

// ---- Order ----

// OrderListView 用于管理端订单列表，JOIN 了下单用户名。
type OrderListView struct {
	ID         uint64     `json:"id" gorm:"column:id"`
	OutTradeNo string     `json:"out_trade_no" gorm:"column:out_trade_no"`
	UserID     uint64     `json:"user_id" gorm:"column:user_id"`
	Username   string     `json:"username" gorm:"column:username"`
	Plan       string     `json:"plan" gorm:"column:plan"`
	Channel    string     `json:"channel" gorm:"column:channel"`
	Amount     int        `json:"amount" gorm:"column:amount"`
	Status     int8       `json:"status" gorm:"column:status"`
	PayChannel string     `json:"pay_channel" gorm:"column:pay_channel"`
	TradeNo    string     `json:"trade_no" gorm:"column:trade_no"`
	PaidAt     *time.Time `json:"paid_at" gorm:"column:paid_at"`
	CreatedAt  time.Time  `json:"created_at" gorm:"column:created_at"`
}

// OrderListReq 管理端订单查询条件
type OrderListReq struct {
	Page     int
	PageSize int
	Keyword  string // 模糊匹配 out_trade_no / trade_no / username
	Status   int8   // -1=不过滤
	Plan     string // 精确匹配套餐；为空时不过滤
}
