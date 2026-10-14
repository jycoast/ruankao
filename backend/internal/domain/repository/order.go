package repository

import (
	"time"

	"software-exam/internal/domain/model"

	"gorm.io/gorm"
)

type OrderRepository interface {
	Create(o *model.Order) error
	FindByOutTradeNo(outTradeNo string) (*model.Order, error)
	MarkPaid(outTradeNo, tradeNo, payChannel string, paidAt time.Time) (bool, error)
	RevertPaid(outTradeNo string) error
	ListByUser(userID uint64, page, pageSize int) ([]model.Order, int64, error)
	ListAll(req OrderListReq) ([]OrderListView, int64, error)
}

type OrderRepositoryImpl struct {
	db *gorm.DB
}

func NewOrderRepository(db *gorm.DB) OrderRepository {
	return &OrderRepositoryImpl{db: db}
}

func (r *OrderRepositoryImpl) Create(o *model.Order) error {
	return r.db.Create(o).Error
}

func (r *OrderRepositoryImpl) FindByOutTradeNo(outTradeNo string) (*model.Order, error) {
	var o model.Order
	err := r.db.Where("out_trade_no = ?", outTradeNo).First(&o).Error
	if err != nil {
		return nil, err
	}
	return &o, nil
}

// MarkPaid 仅在订单仍为待支付(0)时置为已支付(1)。
// 返回 true 表示本次调用真正完成了状态翻转 —— 这是唯一可靠的幂等依据：
// 「先查 status 再更新」在并发回调下两个请求会同时读到 0，双双发放会员。
func (r *OrderRepositoryImpl) MarkPaid(outTradeNo, tradeNo, payChannel string, paidAt time.Time) (bool, error) {
	tx := r.db.Model(&model.Order{}).
		Where("out_trade_no = ? AND status = 0", outTradeNo).
		Updates(map[string]interface{}{
			"status":      1,
			"trade_no":    tradeNo,
			"pay_channel": payChannel,
			"paid_at":     paidAt,
		})
	return tx.RowsAffected > 0, tx.Error
}

// RevertPaid 把订单退回待支付。仅用于「已标记已付但会员发放失败」的补偿：
// 不退回的话订单永远是已付，回调重试会被幂等挡掉，会员再也发不出去。
func (r *OrderRepositoryImpl) RevertPaid(outTradeNo string) error {
	return r.db.Model(&model.Order{}).
		Where("out_trade_no = ? AND status = 1", outTradeNo).
		Updates(map[string]interface{}{"status": 0, "paid_at": nil}).Error
}

func (r *OrderRepositoryImpl) ListByUser(userID uint64, page, pageSize int) ([]model.Order, int64, error) {
	var list []model.Order
	var total int64
	r.db.Model(&model.Order{}).Where("user_id = ?", userID).Count(&total)
	err := r.db.Where("user_id = ?", userID).Order("id DESC").
		Offset((page - 1) * pageSize).Limit(pageSize).Find(&list).Error
	return list, total, err
}

// ListAll 管理端订单列表：JOIN users 取下单人用户名，支持按状态/套餐/关键词筛选。
func (r *OrderRepositoryImpl) ListAll(req OrderListReq) ([]OrderListView, int64, error) {
	var list []OrderListView
	var total int64
	query := r.db.Table("orders").
		Select("orders.*, u.username").
		Joins("LEFT JOIN users u ON u.id = orders.user_id")
	if req.Status != -1 {
		query = query.Where("orders.status = ?", req.Status)
	}
	if req.Plan != "" {
		query = query.Where("orders.plan = ?", req.Plan)
	}
	if req.Keyword != "" {
		kw := "%" + req.Keyword + "%"
		query = query.Where("orders.out_trade_no LIKE ? OR orders.trade_no LIKE ? OR u.username LIKE ?", kw, kw, kw)
	}
	query.Count(&total)
	err := query.Order("orders.id DESC").
		Offset((req.Page - 1) * req.PageSize).Limit(req.PageSize).
		Scan(&list).Error
	return list, total, err
}

// ── 评论区/答疑讨论 (083) ─────────────────────────────────────────
