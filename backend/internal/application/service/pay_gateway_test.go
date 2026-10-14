package service

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"strconv"
	"strings"
	"testing"
	"time"

	"software-exam/config"
	"software-exam/internal/domain/model"
)

// 生成一对测试密钥，并按配置要求编码成不带 PEM 头尾的裸 base64。
func newYTKeys(t *testing.T) (privB64, pubB64 string, priv *rsa.PrivateKey) {
	t.Helper()
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	der, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		t.Fatal(err)
	}
	pubDer, err := x509.MarshalPKIXPublicKey(&priv.PublicKey)
	if err != nil {
		t.Fatal(err)
	}
	return base64.StdEncoding.EncodeToString(der), base64.StdEncoding.EncodeToString(pubDer), priv
}

// newYTFixture 造一个悦通云订单。平台公钥即测试私钥对应的公钥，
// 于是测试可以用同一把私钥模拟「平台」签发回调。
func newYTFixture(t *testing.T) (*PaymentService, *fakeOrderRepo, *fakeMembershipRepo, string) {
	t.Helper()
	privB64, pubB64, _ := newYTKeys(t)
	config.AppConfig.YiPay = config.YiPayConfig{APIURL: "https://pay.test/", PID: "1001", Key: "test-merchant-key"}
	config.AppConfig.LDC = config.LDCConfig{APIURL: "https://credit.test/epay", PID: "2002", Key: "test-ldc-secret"}
	config.AppConfig.YueTongYun = config.YueTongYunConfig{
		APIURL:             "https://yuetongyun.test",
		PID:                "3003",
		MerchantPrivateKey: privB64,
		PlatformPublicKey:  pubB64,
	}
	orderRepo := &fakeOrderRepo{order: &model.Order{
		OutTradeNo: "SE900", UserID: 42, Plan: "month", Channel: ChannelYueTongYun, Amount: 2900, Status: 0,
	}}
	memRepo := &fakeMembershipRepo{}
	planRepo := &fakePlanRepo{plans: []model.MembershipPlan{
		{PlanKey: "month", Name: "月度会员", Days: 30, Amount: 2900, LDCAmount: 2900, Enabled: 1},
	}}
	svc := NewPaymentService(orderRepo, NewMembershipService(memRepo), NewPlanService(planRepo))
	return svc, orderRepo, memRepo, privB64
}

// ytNotifyParams 模拟平台签发的回调（用平台私钥签名）。
func ytNotifyParams(t *testing.T, privB64, outTradeNo, money string, ts int64) map[string]string {
	t.Helper()
	p := map[string]string{
		"pid":          "3003",
		"trade_no":     "T900",
		"out_trade_no": outTradeNo,
		"type":         "alipay",
		"name":         "月度会员",
		"money":        money,
		"trade_status": "TRADE_SUCCESS",
		"timestamp":    strconv.FormatInt(ts, 10),
	}
	sign, err := rsaSignBase64(buildSignContent(p), privB64)
	if err != nil {
		t.Fatal(err)
	}
	p["sign"] = sign
	p["sign_type"] = "RSA"
	return p
}

func TestYueTongYunNotifyGrantsMembership(t *testing.T) {
	svc, _, memRepo, priv := newYTFixture(t)
	p := ytNotifyParams(t, priv, "SE900", "29.00", time.Now().Unix())
	if err := svc.HandleNotify(ChannelYueTongYun, p); err != nil {
		t.Fatalf("合法回调应通过: %v", err)
	}
	if memRepo.grants != 1 {
		t.Fatalf("应发放一次会员，实际 %d", memRepo.grants)
	}
}

func TestYueTongYunNotifyRejectsBadSign(t *testing.T) {
	svc, _, memRepo, priv := newYTFixture(t)
	p := ytNotifyParams(t, priv, "SE900", "29.00", time.Now().Unix())
	p["money"] = "0.01" // 篡改金额后签名即失配
	if err := svc.HandleNotify(ChannelYueTongYun, p); err == nil {
		t.Fatal("篡改后的回调应被拒")
	}
	if memRepo.grants != 0 {
		t.Fatal("验签失败不应发放会员")
	}
}

// 平台会新增回调字段。验签若按固定白名单重建待签串，加字段当天就会全站掉单。
func TestYueTongYunNotifyToleratesUnknownFields(t *testing.T) {
	svc, _, memRepo, priv := newYTFixture(t)
	p := map[string]string{
		"pid": "3003", "trade_no": "T900", "out_trade_no": "SE900",
		"type": "alipay", "name": "月度会员", "money": "29.00",
		"trade_status": "TRADE_SUCCESS",
		"timestamp":    strconv.FormatInt(time.Now().Unix(), 10),
		"api_trade_no": "4200002345", "buyer": "o9uAcc6VlZ", "endtime": "2026-08-02 12:00:00",
	}
	sign, err := rsaSignBase64(buildSignContent(p), priv)
	if err != nil {
		t.Fatal(err)
	}
	p["sign"] = sign
	p["sign_type"] = "RSA"

	if err := svc.HandleNotify(ChannelYueTongYun, p); err != nil {
		t.Fatalf("含未知字段的合法回调应通过: %v", err)
	}
	if memRepo.grants != 1 {
		t.Fatal("应正常发放会员")
	}
}

// 抓到的合法回调不能被无限期重放（官方 SDK 取 300 秒窗口）。
func TestYueTongYunNotifyRejectsStaleTimestamp(t *testing.T) {
	svc, _, memRepo, priv := newYTFixture(t)
	old := time.Now().Add(-10 * time.Minute).Unix()
	p := ytNotifyParams(t, priv, "SE900", "29.00", old)
	if err := svc.HandleNotify(ChannelYueTongYun, p); err == nil {
		t.Fatal("过期回调应被拒，否则合法回调可被无限重放")
	}
	if memRepo.grants != 0 {
		t.Fatal("过期回调不应发放会员")
	}
}

// 悦通云的回调打到易支付订单上必须被拒，反之亦然。
func TestYueTongYunNotifyRejectsCrossChannel(t *testing.T) {
	svc, orderRepo, memRepo, priv := newYTFixture(t)
	orderRepo.order.Channel = ChannelYiPay // 该订单其实走的是易支付

	p := ytNotifyParams(t, priv, "SE900", "29.00", time.Now().Unix())
	if err := svc.HandleNotify(ChannelYueTongYun, p); err == nil {
		t.Fatal("通道不匹配应拒绝")
	}
	if memRepo.grants != 0 {
		t.Fatal("通道不匹配不应发放会员")
	}
}

func TestYueTongYunCreateOrderSignsAndRoutes(t *testing.T) {
	svc, orderRepo, _, _ := newYTFixture(t)
	payURL, _, err := svc.CreateOrder(42, "month", "yt_alipay")
	if err != nil {
		t.Fatalf("下单失败: %v", err)
	}
	if orderRepo.order.Channel != ChannelYueTongYun {
		t.Fatalf("订单通道应为悦通云，实际 %s", orderRepo.order.Channel)
	}
	if orderRepo.order.Amount != 2900 {
		t.Fatalf("应按人民币价下单，实际 %d", orderRepo.order.Amount)
	}
	// 前端的 yt_alipay 是为了全站唯一，网关只认去前缀的 alipay
	if !strings.Contains(payURL, "type=alipay") {
		t.Fatalf("应以 type=alipay 提交，实际 %s", payURL)
	}
	if !strings.Contains(payURL, "sign_type=RSA") {
		t.Fatalf("应带 sign_type=RSA，实际 %s", payURL)
	}
	if !strings.HasPrefix(payURL, "https://yuetongyun.test/api/pay/submit?") {
		t.Fatalf("下单地址不对: %s", payURL)
	}
}

// 未配置的通道不该出现在支付方式里，否则用户点了才发现失败。
func TestAvailableChannelsSkipsUnconfigured(t *testing.T) {
	svc, _, _, _ := newYTFixture(t)
	config.AppConfig.YueTongYun.PlatformPublicKey = "" // 少了验签公钥即视为未配好

	for _, m := range svc.AvailableChannels() {
		if strings.HasPrefix(m.Type, "yt_") {
			t.Fatalf("缺少平台公钥时不应展示悦通云支付方式，却出现了 %s", m.Type)
		}
	}
}
