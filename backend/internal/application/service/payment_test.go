package service

import (
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"software-exam/config"
	"software-exam/internal/domain/model"
	"software-exam/internal/domain/repository"
)

// 内存版 order 仓储，MarkPaid 用锁模拟数据库条件更新的原子性。
type fakeOrderRepo struct {
	mu    sync.Mutex
	order *model.Order
}

func (f *fakeOrderRepo) Create(o *model.Order) error { f.order = o; return nil }

func (f *fakeOrderRepo) FindByOutTradeNo(no string) (*model.Order, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.order == nil || f.order.OutTradeNo != no {
		return nil, errors.New("not found")
	}
	cp := *f.order
	return &cp, nil
}

func (f *fakeOrderRepo) MarkPaid(no, tradeNo, channel string, paidAt time.Time) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.order == nil || f.order.OutTradeNo != no || f.order.Status != 0 {
		return false, nil
	}
	f.order.Status = 1
	f.order.TradeNo = tradeNo
	f.order.PayChannel = channel
	f.order.PaidAt = &paidAt
	return true, nil
}

func (f *fakeOrderRepo) RevertPaid(no string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.order != nil && f.order.OutTradeNo == no && f.order.Status == 1 {
		f.order.Status = 0
		f.order.PaidAt = nil
	}
	return nil
}

func (f *fakeOrderRepo) ListByUser(uint64, int, int) ([]model.Order, int64, error) {
	panic("unused")
}

func (f *fakeOrderRepo) ListAll(repository.OrderListReq) ([]repository.OrderListView, int64, error) {
	panic("unused")
}

type fakeMembershipRepo struct {
	mu      sync.Mutex
	m       *model.Membership
	grants  int
	failNow bool
}

func (f *fakeMembershipRepo) FindByUserID(userID uint64) (*model.Membership, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.m == nil || f.m.UserID != userID {
		return nil, errors.New("not found")
	}
	cp := *f.m
	return &cp, nil
}

func (f *fakeMembershipRepo) FindByUserIDs(ids []uint64) ([]model.Membership, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.m == nil {
		return nil, nil
	}
	for _, id := range ids {
		if f.m.UserID == id {
			cp := *f.m
			return []model.Membership{cp}, nil
		}
	}
	return nil, nil
}

func (f *fakeMembershipRepo) ExistsByUserID(userID uint64) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.m != nil && f.m.UserID == userID, nil
}

func (f *fakeMembershipRepo) Upsert(m *model.Membership) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.failNow {
		return errors.New("db down")
	}
	f.grants++
	cp := *m
	f.m = &cp
	return nil
}

// 内存版套餐仓储。
type fakePlanRepo struct {
	plans []model.MembershipPlan
}

func (f *fakePlanRepo) List() ([]model.MembershipPlan, error) { return f.plans, nil }

func (f *fakePlanRepo) FindByKey(key string) (*model.MembershipPlan, error) {
	for i := range f.plans {
		if f.plans[i].PlanKey == key {
			cp := f.plans[i]
			return &cp, nil
		}
	}
	return nil, errors.New("not found")
}

func (f *fakePlanRepo) Update(p *model.MembershipPlan) error {
	for i := range f.plans {
		if f.plans[i].PlanKey == p.PlanKey {
			f.plans[i] = *p
			return nil
		}
	}
	return errors.New("not found")
}

func notifyParams(t *testing.T, outTradeNo, money string) map[string]string {
	t.Helper()
	p := map[string]string{
		"pid":          "1001",
		"trade_no":     "T123",
		"out_trade_no": outTradeNo,
		"type":         "alipay",
		"name":         "月度会员",
		"money":        money,
		"trade_status": "TRADE_SUCCESS",
	}
	p["sign"] = buildYiPaySign(p, config.AppConfig.YiPay.Key)
	p["sign_type"] = "MD5"
	return p
}

func newPayFixture(t *testing.T) (*PaymentService, *fakeOrderRepo, *fakeMembershipRepo) {
	t.Helper()
	config.AppConfig.YiPay = config.YiPayConfig{APIURL: "https://pay.test/", PID: "1001", Key: "test-merchant-key"}
	config.AppConfig.LDC = config.LDCConfig{APIURL: "https://credit.test/epay", PID: "2002", Key: "test-ldc-secret"}
	orderRepo := &fakeOrderRepo{order: &model.Order{
		OutTradeNo: "SE001", UserID: 42, Plan: "month", Channel: ChannelYiPay, Amount: 2900, Status: 0,
	}}
	memRepo := &fakeMembershipRepo{}
	planRepo := &fakePlanRepo{plans: []model.MembershipPlan{
		{PlanKey: "month", Name: "月度会员", Days: 30, Amount: 2900, LDCAmount: 2900, Enabled: 1},
	}}
	svc := NewPaymentService(orderRepo, NewMembershipService(memRepo), NewPlanService(planRepo))
	return svc, orderRepo, memRepo
}

// 易支付会重试回调，两个请求可能同时到达。
// 原实现「先读 status 再更新」会让两者都读到 0，双倍发会员。
func TestNotifyConcurrentCallbacksGrantOnce(t *testing.T) {
	svc, _, memRepo := newPayFixture(t)
	params := notifyParams(t, "SE001", "29.00")

	const n = 8
	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func() {
			defer wg.Done()
			if err := svc.HandleNotify(ChannelYiPay, params); err != nil {
				t.Errorf("notify failed: %v", err)
			}
		}()
	}
	wg.Wait()

	if memRepo.grants != 1 {
		t.Fatalf("会员应只发放一次，实际 %d 次", memRepo.grants)
	}
}

// 网关可能回传 "29" 而非 "29.00"，字符串比较会误判金额不符导致掉单。
func TestNotifyAcceptsUnpaddedAmount(t *testing.T) {
	for _, money := range []string{"29", "29.0", "29.00", "29.000"} {
		svc, _, memRepo := newPayFixture(t)
		if err := svc.HandleNotify(ChannelYiPay, notifyParams(t, "SE001", money)); err != nil {
			t.Fatalf("money=%q 应被接受: %v", money, err)
		}
		if memRepo.grants != 1 {
			t.Fatalf("money=%q 未发放会员", money)
		}
	}
}

func TestNotifyRejectsWrongAmount(t *testing.T) {
	svc, _, memRepo := newPayFixture(t)
	if err := svc.HandleNotify(ChannelYiPay, notifyParams(t, "SE001", "0.01")); err == nil {
		t.Fatal("金额被篡改应拒绝")
	}
	if memRepo.grants != 0 {
		t.Fatal("金额不符时不应发放会员")
	}
}

// 发放失败必须把订单退回待支付，否则重试被幂等挡掉，钱收了会员发不出。
func TestNotifyRollsBackOrderWhenGrantFails(t *testing.T) {
	svc, orderRepo, memRepo := newPayFixture(t)
	memRepo.failNow = true

	params := notifyParams(t, "SE001", "29.00")
	if err := svc.HandleNotify(ChannelYiPay, params); err == nil {
		t.Fatal("发放失败应返回错误，让网关重试")
	}
	if orderRepo.order.Status != 0 {
		t.Fatalf("订单应退回待支付，实际 status=%d", orderRepo.order.Status)
	}

	// 网关重试：此时数据库恢复，应能补发成功
	memRepo.failNow = false
	if err := svc.HandleNotify(ChannelYiPay, params); err != nil {
		t.Fatalf("重试应成功: %v", err)
	}
	if memRepo.grants != 1 {
		t.Fatalf("重试后应发放会员，实际 %d 次", memRepo.grants)
	}
}

func TestNotifyRejectsBadSign(t *testing.T) {
	svc, _, memRepo := newPayFixture(t)
	params := notifyParams(t, "SE001", "29.00")
	params["sign"] = "deadbeef"
	if err := svc.HandleNotify(ChannelYiPay, params); err == nil {
		t.Fatal("签名错误应拒绝")
	}
	if memRepo.grants != 0 {
		t.Fatal("签名错误时不应发放会员")
	}
}

// 两条通道密钥不同。拿 LDC 的密钥签一份回调打到易支付订单上必须被拒 ——
// 否则任一通道的密钥泄露就等于所有订单都能被伪造成已支付。
func TestNotifyRejectsCrossChannel(t *testing.T) {
	svc, _, memRepo := newPayFixture(t)

	p := map[string]string{
		"pid": "2002", "trade_no": "T999", "out_trade_no": "SE001",
		"type": "epay", "name": "月度会员", "money": "29.00",
		"trade_status": "TRADE_SUCCESS",
	}
	p["sign"] = buildYiPaySign(p, config.AppConfig.LDC.Key)

	if err := svc.HandleNotify(ChannelLDC, p); err == nil {
		t.Fatal("LDC 回调打到易支付订单上应拒绝")
	}
	if memRepo.grants != 0 {
		t.Fatal("通道不匹配时不应发放会员")
	}
}

// 下单后管理员做活动改价，回调必须按订单里的价格快照校验。
// 若按当前套餐价比对，活动前下的单会全部被判金额不符而掉单。
func TestNotifyUsesOrderPriceSnapshotNotCurrentPrice(t *testing.T) {
	svc, _, memRepo := newPayFixture(t)
	params := notifyParams(t, "SE001", "29.00")

	// 用户已按 ¥29 下单并付款，管理员此时把月度会员降到 ¥9
	if err := svc.planSvc.UpdatePrice("month", "月度会员", 900, 900, 1, 1); err != nil {
		t.Fatalf("调价失败: %v", err)
	}

	if err := svc.HandleNotify(ChannelYiPay, params); err != nil {
		t.Fatalf("调价不应影响已下单订单的结算: %v", err)
	}
	if memRepo.grants != 1 {
		t.Fatal("调价后老订单仍应正常发放会员")
	}
}

// 下架的套餐不能再新购，但已售卡密/存量订单仍要能按原天数结算。
func TestCreateOrderRejectsDisabledPlan(t *testing.T) {
	svc, _, _ := newPayFixture(t)
	if err := svc.planSvc.UpdatePrice("month", "月度会员", 2900, 2900, 1, 0); err != nil {
		t.Fatalf("下架失败: %v", err)
	}
	if _, _, err := svc.CreateOrder(42, "month", "alipay"); err == nil {
		t.Fatal("已下架套餐不应可下单")
	}
	if _, ok := svc.planSvc.Find("month"); !ok {
		t.Fatal("已下架套餐仍应可查到，否则存量订单无法结算")
	}
}

// LDC 通道按积分价计，与人民币价互不影响（活动期可只打折其中一条）。
func TestCreateOrderLDCUsesLDCPrice(t *testing.T) {
	svc, orderRepo, _ := newPayFixture(t)
	if err := svc.planSvc.UpdatePrice("month", "月度会员", 2900, 1500, 1, 1); err != nil {
		t.Fatalf("调价失败: %v", err)
	}

	payURL, _, err := svc.CreateOrder(42, "month", "ldcpay")
	if err != nil {
		t.Fatalf("LDC 下单失败: %v", err)
	}
	if orderRepo.order.Amount != 1500 || orderRepo.order.Channel != ChannelLDC {
		t.Fatalf("订单应记 LDC 积分价与通道，实际 amount=%d channel=%s",
			orderRepo.order.Amount, orderRepo.order.Channel)
	}
	// LDC 的易支付兼容接口只认 type=epay，传 ldcpay 会被网关拒
	if !strings.Contains(payURL, "type=epay") {
		t.Fatalf("LDC 下单应以 type=epay 提交，实际 URL: %s", payURL)
	}
	if !strings.Contains(payURL, "money=15.00") {
		t.Fatalf("LDC 下单金额应为 15.00，实际 URL: %s", payURL)
	}
}

// ---- MembershipService.Status：类型化出参，非会员 expire_at 为 JSON null ----

// 非会员：is_member=false、expire_at=null、plan=""、days_left=0。
// JSON key 逐字保持 is_member/expire_at/plan/days_left。
func TestMembershipStatus_NotMember_ExpireAtNull(t *testing.T) {
	memRepo := &fakeMembershipRepo{}
	svc := NewMembershipService(memRepo)

	st := svc.Status(42)
	if st.IsMember {
		t.Fatalf("非会员 IsMember 应为 false，得到 %+v", st)
	}
	if st.ExpireAt != nil {
		t.Fatalf("非会员 ExpireAt 应为 nil（JSON null），得到 %v", st.ExpireAt)
	}
	if st.Plan != "" || st.DaysLeft != 0 {
		t.Fatalf("非会员 plan/days_left 应为空/0，得到 %+v", st)
	}

	b, err := json.Marshal(st)
	if err != nil {
		t.Fatalf("marshal 失败: %v", err)
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("unmarshal 失败: %v", err)
	}
	for _, k := range []string{"is_member", "expire_at", "plan", "days_left"} {
		if _, ok := m[k]; !ok {
			t.Fatalf("JSON 缺 key %q: %s", k, b)
		}
	}
	if string(m["expire_at"]) != "null" {
		t.Fatalf("非会员 expire_at 应序列化为 null，得到 %s", m["expire_at"])
	}
}

// 有效会员：is_member=true、expire_at 非 null、days_left 按剩余天数向上取整。
func TestMembershipStatus_ActiveMember(t *testing.T) {
	memRepo := &fakeMembershipRepo{m: &model.Membership{
		UserID: 42, Plan: "month", ExpireAt: time.Now().Add(48 * time.Hour),
	}}
	svc := NewMembershipService(memRepo)

	st := svc.Status(42)
	if !st.IsMember {
		t.Fatalf("有效期会员 IsMember 应为 true，得到 %+v", st)
	}
	if st.ExpireAt == nil {
		t.Fatal("有效期会员 ExpireAt 不应为 nil")
	}
	if st.Plan != "month" {
		t.Fatalf("Plan 应透传 month，得到 %q", st.Plan)
	}
	if st.DaysLeft != 3 { // 48h → int(48/24)+1 = 3
		t.Fatalf("48h 剩余应算 3 天，得到 %d", st.DaysLeft)
	}
}

// 已过期会员：is_member=false（不再展示会员态），days_left=0。
func TestMembershipStatus_ExpiredMember(t *testing.T) {
	memRepo := &fakeMembershipRepo{m: &model.Membership{
		UserID: 42, Plan: "month", ExpireAt: time.Now().Add(-24 * time.Hour),
	}}
	svc := NewMembershipService(memRepo)

	st := svc.Status(42)
	if st.IsMember {
		t.Fatalf("过期会员 IsMember 应为 false，得到 %+v", st)
	}
	if st.DaysLeft != 0 {
		t.Fatalf("过期会员 days_left 应为 0，得到 %d", st.DaysLeft)
	}
}
