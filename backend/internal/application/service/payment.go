package service

import (
	"errors"
	"fmt"
	"log"
	"math"
	"strconv"
	"strings"
	"time"

	"software-exam/internal/domain/model"
	"software-exam/internal/domain/repository"
)

// PlanDef 会员套餐定义（对外的只读视图）。
// 价格由管理员在后台调整，落库在 membership_plans，不再硬编码。
type PlanDef struct {
	Key       string `json:"key"`
	Name      string `json:"name"`
	Days      int    `json:"days"`
	Amount    int    `json:"amount"`     // 人民币，单位：分
	LDCAmount int    `json:"ldc_amount"` // LDC 积分，单位：0.01 积分
	Enabled   int8   `json:"enabled"`
}

func planFromModel(p *model.MembershipPlan) PlanDef {
	return PlanDef{
		Key:       p.PlanKey,
		Name:      p.Name,
		Days:      p.Days,
		Amount:    p.Amount,
		LDCAmount: p.LDCAmount,
		Enabled:   p.Enabled,
	}
}

// PlanService 套餐读取与后台调价。
type PlanService struct {
	repo repository.MembershipPlanRepository
}

func NewPlanService(repo repository.MembershipPlanRepository) *PlanService {
	return &PlanService{repo: repo}
}

// Find 按 key 取套餐，包括已下架的 ——
// 存量订单与已售卡密引用的套餐可能已下架，结算时仍必须能查到天数。
func (s *PlanService) Find(key string) (PlanDef, bool) {
	p, err := s.repo.FindByKey(key)
	if err != nil || p == nil {
		return PlanDef{}, false
	}
	return planFromModel(p), true
}

// ListSellable 前台展示用：仅在售套餐。
func (s *PlanService) ListSellable() ([]PlanDef, error) {
	all, err := s.repo.List()
	if err != nil {
		return nil, err
	}
	out := make([]PlanDef, 0, len(all))
	for i := range all {
		if all[i].Enabled == 1 {
			out = append(out, planFromModel(&all[i]))
		}
	}
	return out, nil
}

// ListAll 后台管理用：含已下架套餐。
func (s *PlanService) ListAll() ([]model.MembershipPlan, error) {
	return s.repo.List()
}

// UpdatePrice 后台调价。只动价格与展示字段：
// days 决定已售卡密兑换的时长，plan_key 是订单/卡密的关联键，改了会让存量记录对不上。
func (s *PlanService) UpdatePrice(key, name string, amount, ldcAmount, sortOrder int, enabled int8) error {
	if amount < 0 || ldcAmount < 0 {
		return errors.New("价格不能为负")
	}
	p, err := s.repo.FindByKey(key)
	if err != nil || p == nil {
		return errors.New("套餐不存在")
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return errors.New("套餐名称不能为空")
	}
	p.Name = name
	p.Amount = amount
	p.LDCAmount = ldcAmount
	p.SortOrder = sortOrder
	p.Enabled = enabled
	return s.repo.Update(p)
}

// ---------------- MembershipService ----------------

type MembershipService struct {
	repo repository.MembershipRepository
}

func NewMembershipService(repo repository.MembershipRepository) *MembershipService {
	return &MembershipService{repo: repo}
}

// ExistsByUserID 该用户是否有过会员记录（含已过期）。首登赠送用它做幂等判定。
func (s *MembershipService) ExistsByUserID(userID uint64) bool {
	exists, err := s.repo.ExistsByUserID(userID)
	if err != nil {
		return false // 出错保守视为已存在，避免异常情况下重复发放
	}
	return exists
}

// IsActive 当前用户是否为有效会员
func (s *MembershipService) IsActive(userID uint64) bool {
	m, err := s.repo.FindByUserID(userID)
	if err != nil || m == nil {
		return false
	}
	return m.ExpireAt.After(time.Now())
}

// MembershipStatus 会员状态出参。ExpireAt 用 *time.Time：非会员/查不到记录时为 nil，
// JSON 序列化恰好输出 null，与前端「无到期时间」语义一致。
type MembershipStatus struct {
	IsMember bool       `json:"is_member"`
	ExpireAt *time.Time `json:"expire_at"`
	Plan     string     `json:"plan"`
	DaysLeft int        `json:"days_left"`
}

// Status 会员状态，供前端展示。days_left 用于到期提醒（已过期为 0）。
func (s *MembershipService) Status(userID uint64) MembershipStatus {
	m, err := s.repo.FindByUserID(userID)
	if err != nil || m == nil {
		return MembershipStatus{IsMember: false, ExpireAt: nil, Plan: "", DaysLeft: 0}
	}
	now := time.Now()
	active := m.ExpireAt.After(now)
	daysLeft := 0
	if active {
		daysLeft = int(m.ExpireAt.Sub(now).Hours()/24) + 1
	}
	return MembershipStatus{
		IsMember: active,
		ExpireAt: &m.ExpireAt,
		Plan:     m.Plan,
		DaysLeft: daysLeft,
	}
}

// Grant 发放/续期会员：已是有效会员则从到期时间叠加，否则从当前时间起算。
func (s *MembershipService) Grant(userID uint64, plan string, days int) error {
	if days <= 0 {
		return errors.New("天数必须大于 0")
	}
	now := time.Now()
	base := now
	if m, err := s.repo.FindByUserID(userID); err == nil && m != nil && m.ExpireAt.After(now) {
		base = m.ExpireAt
	}
	return s.repo.Upsert(&model.Membership{
		UserID:   userID,
		Plan:     plan,
		ExpireAt: base.AddDate(0, 0, days),
	})
}

// ---------------- PaymentService ----------------

type PaymentService struct {
	orderRepo     repository.OrderRepository
	membershipSvc *MembershipService
	planSvc       *PlanService
}

func NewPaymentService(orderRepo repository.OrderRepository, membershipSvc *MembershipService, planSvc *PlanService) *PaymentService {
	return &PaymentService{orderRepo: orderRepo, membershipSvc: membershipSvc, planSvc: planSvc}
}

func (s *PaymentService) ListPlans() ([]PlanDef, error) { return s.planSvc.ListSellable() }

// PayChannel 一个可用的支付方式，供前端渲染支付按钮。
type PayChannel struct {
	Type string `json:"type"` // 传回 CreateOrder 的 type，全站唯一
	Name string `json:"name"`
	// Credit 该方式以积分而非人民币计价。前端据此显示「N 积分」而不是「¥N」，
	// 由后端下发而非前端按 type 硬猜，新增积分类渠道时前端无需改。
	Credit bool `json:"credit"`
}

// AvailableChannels 只返回已配置好商户信息的通道。
// 没配的通道不露出来，否则用户点了才报「支付未配置」。
func (s *PaymentService) AvailableChannels() []PayChannel {
	out := []PayChannel{}
	for _, g := range gatewayRegistry {
		if g.Configured() {
			out = append(out, g.Methods()...)
		}
	}
	return out
}

// CreateOrder 创建订单并返回支付页面跳转 URL。
//
// payType 是前端选中的支付方式（全站唯一），通道由它反查得到 ——
// 不让前端自己声明通道，避免用 A 通道的价格下 B 通道的单。
// 价格按下单当刻的套餐价存进订单做快照 —— 管理员随后调价不能影响已下单的金额校验。
func (s *PaymentService) CreateOrder(userID uint64, planKey, payType string) (payURL, outTradeNo string, err error) {
	plan, ok := s.planSvc.Find(planKey)
	if !ok {
		return "", "", errors.New("套餐不存在")
	}
	if plan.Enabled != 1 {
		return "", "", errors.New("该套餐已下架")
	}

	gw, err := gatewayForMethod(payType)
	if err != nil {
		return "", "", err
	}
	amount := gw.AmountOf(plan, payType)
	if amount <= 0 {
		return "", "", errors.New("该套餐暂不支持此支付方式")
	}

	outTradeNo = fmt.Sprintf("SE%d%04d", time.Now().UnixMilli(), userID%10000)
	order := &model.Order{
		OutTradeNo: outTradeNo,
		UserID:     userID,
		Plan:       plan.Key,
		Channel:    gw.Channel(),
		Amount:     amount,
		Status:     0,
	}
	if err = s.orderRepo.Create(order); err != nil {
		return "", "", fmt.Errorf("创建订单失败: %w", err)
	}

	payURL, err = gw.BuildPayURL(payType, outTradeNo, plan.Name, amount)
	if err != nil {
		return "", "", err
	}
	return payURL, outTradeNo, nil
}

// HandleNotify 处理支付异步回调。channel 由回调路由决定（各通道密钥不同，
// 必须用对应密钥验签，不能让回调方自己声明自己走哪条通道）。返回 nil 表示应答 "success"。
//
// 幂等完全依赖 MarkPaid 的条件更新：只有把 status 从 0 翻成 1 的那一次调用
// 才会去发放会员。不能改成「先查 status==1 就 return」——并发的两个回调
// 会同时读到 0，双双通过检查，双倍发卡。
func (s *PaymentService) HandleNotify(channel string, params map[string]string) error {
	gw, err := gatewayFor(channel)
	if err != nil {
		return err
	}

	// 1. 验签（含通道自有的时效等约束）
	if err := gw.VerifyNotify(params); err != nil {
		return err
	}
	// 2. 支付状态
	if params["trade_status"] != "TRADE_SUCCESS" {
		return errors.New("交易未成功")
	}

	outTradeNo := params["out_trade_no"]
	order, err := s.orderRepo.FindByOutTradeNo(outTradeNo)
	if err != nil || order == nil {
		return errors.New("订单不存在")
	}
	// 3. 通道核对：拿 A 通道的密钥去签 B 通道的订单不该被接受。
	if order.Channel != channel {
		return errors.New("支付通道不匹配")
	}
	// 4. 金额核对（防篡改）。按「分」比较，避免 "29" 与 "29.00" 被判为不等。
	//    比的是订单里的价格快照，不是当前套餐价 —— 否则活动调价会误杀活动前的订单。
	paid, err := parseYuanToCents(params["money"])
	if err != nil {
		return errors.New("金额格式非法")
	}
	if paid != order.Amount {
		return errors.New("金额不匹配")
	}

	plan, ok := s.planSvc.Find(order.Plan)
	if !ok {
		return errors.New("套餐已下线")
	}

	// 5. 抢占式标记已支付
	first, err := s.orderRepo.MarkPaid(outTradeNo, params["trade_no"], params["type"], time.Now())
	if err != nil {
		return fmt.Errorf("更新订单失败: %w", err)
	}
	if !first {
		return nil // 已被其它回调处理过
	}
	// 6. 发放会员。失败必须把订单退回待支付，否则重试会被幂等挡掉，
	// 用户付了钱却永远拿不到会员，且日志之外无人知晓。
	if err := s.membershipSvc.Grant(order.UserID, plan.Key, plan.Days); err != nil {
		if rErr := s.orderRepo.RevertPaid(outTradeNo); rErr != nil {
			log.Printf("[pay] 订单 %s 发放会员失败且回滚失败，需人工介入: grant=%v revert=%v", outTradeNo, err, rErr)
		}
		return fmt.Errorf("发放会员失败: %w", err)
	}
	return nil
}

// parseYuanToCents 把易支付回传的元金额（"29" / "29.0" / "29.00"）转成分。
func parseYuanToCents(money string) (int, error) {
	money = strings.TrimSpace(money)
	if money == "" {
		return 0, errors.New("金额为空")
	}
	yuan, err := strconv.ParseFloat(money, 64)
	if err != nil {
		return 0, err
	}
	return int(math.Round(yuan * 100)), nil
}

// QueryOrder 供前端支付完成后轮询订单状态
func (s *PaymentService) QueryOrder(userID uint64, outTradeNo string) (*model.Order, error) {
	o, err := s.orderRepo.FindByOutTradeNo(outTradeNo)
	if err != nil || o == nil {
		return nil, errors.New("订单不存在")
	}
	if o.UserID != userID {
		return nil, errors.New("无权查看该订单")
	}
	return o, nil
}

// ListOrders 用户自己的订单记录（分页）
func (s *PaymentService) ListOrders(userID uint64, page, pageSize int) ([]model.Order, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 50 {
		pageSize = 20
	}
	return s.orderRepo.ListByUser(userID, page, pageSize)
}

// AdminOrderListReq 管理端订单列表查询条件（service 层入参，避免 handler 直接依赖 repository）。
type AdminOrderListReq struct {
	Page     int
	PageSize int
	Keyword  string // 模糊匹配 out_trade_no / trade_no / username
	Status   int8   // -1=不过滤
	Plan     string // 精确匹配套餐；为空时不过滤
}

// ListAllOrders 后台管理用：管理端订单列表，含下单用户名，支持按状态/套餐/关键词筛选。
func (s *PaymentService) ListAllOrders(req AdminOrderListReq) ([]repository.OrderListView, int64, error) {
	if req.Page < 1 {
		req.Page = 1
	}
	if req.PageSize < 1 || req.PageSize > 200 {
		req.PageSize = 20
	}
	return s.orderRepo.ListAll(repository.OrderListReq(req))
}
