package service

import (
	"crypto"
	"crypto/md5"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"software-exam/config"
)

// 支付通道标识。持久化进 orders.channel，改动即等于让存量订单对不上号。
const (
	ChannelYiPay      = "yipay"
	ChannelLDC        = "ldc"
	ChannelYueTongYun = "yuetongyun"
)

// PayGateway 一条支付通道。新接一家支付只需实现本接口并在 gatewayRegistry 注册，
// 下单/回调/前端展示三处都不必再改。
//
// 各家网关的差异集中在两点，也正是接口要抽象掉的东西：
//   - 签名算法不同（易支付系是 MD5 共享密钥，悦通云是 SHA256WithRSA 非对称）
//   - 下单参数与跳转方式不同
type PayGateway interface {
	// Channel 通道标识，写入 orders.channel。
	Channel() string

	// Configured 商户信息是否配全。没配的通道不会出现在前端支付方式里。
	Configured() bool

	// Methods 本通道对外提供的支付方式（前端单选项）。
	// 同一通道可给出多个，如易支付的支付宝/微信。
	Methods() []PayChannel

	// AmountOf 该支付方式应结算的金额（单位：分 / 0.01 积分）。
	// 积分通道与人民币通道量纲不同，价格也各自独立，故由通道自己决定取哪个价。
	AmountOf(plan PlanDef, method string) int

	// BuildPayURL 生成跳转支付的 URL。
	BuildPayURL(method, outTradeNo, subject string, amount int) (string, error)

	// VerifyNotify 校验异步回调的真实性（验签 + 通道自有的时效等约束）。
	VerifyNotify(params map[string]string) error
}

// gatewayRegistry 所有已接入的通道。新增渠道在此追加一行即可。
var gatewayRegistry = []PayGateway{
	yiPayGateway{},
	ldcGateway{},
	yueTongYunGateway{},
}

func gatewayFor(channel string) (PayGateway, error) {
	for _, g := range gatewayRegistry {
		if g.Channel() == channel {
			if !g.Configured() {
				return nil, errors.New("支付未配置，请联系管理员")
			}
			return g, nil
		}
	}
	return nil, errors.New("不支持的支付通道")
}

// gatewayForMethod 按前端选的支付方式反查通道。
// 支付方式全局唯一，因此前端只需回传 type，不必也不该自己声明走哪条通道。
func gatewayForMethod(method string) (PayGateway, error) {
	for _, g := range gatewayRegistry {
		if !g.Configured() {
			continue
		}
		for _, m := range g.Methods() {
			if m.Type == method {
				return g, nil
			}
		}
	}
	return nil, errors.New("不支持的支付方式")
}

// ---------------- 易支付系（MD5 共享密钥） ----------------

// epayBase 彩虹易支付协议的公共实现。LDC 的兼容接口同构，只是密钥、
// 下单路径与 type 取值不同，故以字段区分而非各写一份。
type epayBase struct {
	apiURL     string
	pid        string
	key        string
	notifyURL  string
	returnURL  string
	submitPath string // 相对 apiURL 的下单路径，两家网关不同
	gatewayTyp string // 传给网关的 type（LDC 的兼容接口只认 epay），空则透传前端选的
}

func (e epayBase) configured() bool {
	return e.apiURL != "" && e.pid != "" && e.key != ""
}

func (e epayBase) buildPayURL(method, outTradeNo, subject string, amount int) (string, error) {
	typ := method
	if e.gatewayTyp != "" {
		typ = e.gatewayTyp
	}
	params := map[string]string{
		"pid":          e.pid,
		"type":         typ,
		"out_trade_no": outTradeNo,
		"notify_url":   e.notifyURL,
		"return_url":   e.returnURL,
		"name":         subject,
		"money":        formatYuan(amount),
	}
	params["sign"] = buildYiPaySign(params, e.key)
	params["sign_type"] = "MD5"
	return strings.TrimRight(e.apiURL, "/") + e.submitPath + "?" + buildQuery(params), nil
}

func (e epayBase) verifyNotify(params map[string]string) error {
	if !verifyYiPaySign(params, e.key) {
		return errors.New("签名校验失败")
	}
	return nil
}

// yiPayGateway 易支付：人民币，支付宝/微信。
type yiPayGateway struct{}

func (yiPayGateway) base() epayBase {
	y := config.AppConfig.YiPay
	return epayBase{y.APIURL, y.PID, y.Key, y.NotifyURL, y.ReturnURL, "/submit.php", ""}
}

func (g yiPayGateway) Channel() string  { return ChannelYiPay }
func (g yiPayGateway) Configured() bool { return g.base().configured() }

func (g yiPayGateway) Methods() []PayChannel {
	return []PayChannel{
		{Type: "alipay", Name: "支付宝"},
		{Type: "wxpay", Name: "微信支付"},
	}
}

func (g yiPayGateway) AmountOf(plan PlanDef, _ string) int { return plan.Amount }

func (g yiPayGateway) BuildPayURL(method, outTradeNo, subject string, amount int) (string, error) {
	return g.base().buildPayURL(method, outTradeNo, subject, amount)
}

func (g yiPayGateway) VerifyNotify(params map[string]string) error {
	return g.base().verifyNotify(params)
}

// ldcGateway LINUX DO Credit：社区积分，走其易支付兼容接口。
type ldcGateway struct{}

func (ldcGateway) base() epayBase {
	l := config.AppConfig.LDC
	return epayBase{l.APIURL, l.PID, l.Key, l.NotifyURL, l.ReturnURL, "/pay/submit.php", "epay"}
}

func (g ldcGateway) Channel() string  { return ChannelLDC }
func (g ldcGateway) Configured() bool { return g.base().configured() }

func (g ldcGateway) Methods() []PayChannel {
	return []PayChannel{{Type: "ldcpay", Name: "LINUX DO 积分", Credit: true}}
}

// 积分价与人民币价互相独立，活动期可只打折其中一条。
func (g ldcGateway) AmountOf(plan PlanDef, _ string) int { return plan.LDCAmount }

func (g ldcGateway) BuildPayURL(method, outTradeNo, subject string, amount int) (string, error) {
	return g.base().buildPayURL(method, outTradeNo, subject, amount)
}

func (g ldcGateway) VerifyNotify(params map[string]string) error {
	return g.base().verifyNotify(params)
}

// ---------------- 悦通云（SHA256WithRSA） ----------------

// ytNotifyMaxSkew 回调时间戳容差。官方 SDK 取 300 秒：
// 没有这道闸，一份被抓到的合法回调可以被无限期重放。
const ytNotifyMaxSkew = 300 * time.Second

type yueTongYunGateway struct{}

func (yueTongYunGateway) conf() config.YueTongYunConfig { return config.AppConfig.YueTongYun }

func (g yueTongYunGateway) Channel() string { return ChannelYueTongYun }

func (g yueTongYunGateway) Configured() bool {
	c := g.conf()
	// 私钥用于下单签名，公钥用于回调验签，缺一不可：
	// 少了公钥就只能收单不能安全地收款。
	return c.APIURL != "" && c.PID != "" && c.MerchantPrivateKey != "" && c.PlatformPublicKey != ""
}

func (g yueTongYunGateway) Methods() []PayChannel {
	return []PayChannel{
		{Type: "yt_alipay", Name: "支付宝"},
		{Type: "yt_wxpay", Name: "微信支付"},
		{Type: "yt_qqpay", Name: "QQ钱包"},
	}
}

func (g yueTongYunGateway) AmountOf(plan PlanDef, _ string) int { return plan.Amount }

// ytMethodToType 前端支付方式 → 网关 type。
// 前缀是为了让支付方式在全站唯一（易支付也有 alipay/wxpay），网关只认去前缀的值。
var ytMethodToType = map[string]string{
	"yt_alipay": "alipay",
	"yt_wxpay":  "wxpay",
	"yt_qqpay":  "qqpay",
}

func (g yueTongYunGateway) BuildPayURL(method, outTradeNo, subject string, amount int) (string, error) {
	c := g.conf()
	typ, ok := ytMethodToType[method]
	if !ok {
		return "", errors.New("不支持的支付方式")
	}
	params := map[string]string{
		"pid":          c.PID,
		"type":         typ,
		"out_trade_no": outTradeNo,
		"notify_url":   c.NotifyURL,
		"return_url":   c.ReturnURL,
		"name":         subject,
		"money":        formatYuan(amount),
		"timestamp":    strconv.FormatInt(time.Now().Unix(), 10),
	}
	sign, err := rsaSignBase64(buildSignContent(params), c.MerchantPrivateKey)
	if err != nil {
		return "", err
	}
	params["sign"] = sign
	params["sign_type"] = "RSA"
	return strings.TrimRight(c.APIURL, "/") + "/api/pay/submit?" + buildQuery(params), nil
}

func (g yueTongYunGateway) VerifyNotify(params map[string]string) error {
	c := g.conf()
	// 时间戳先于验签检查：过期的回调即便签名合法也必须拒。
	ts, err := strconv.ParseInt(params["timestamp"], 10, 64)
	if err != nil {
		return errors.New("回调时间戳非法")
	}
	if d := time.Since(time.Unix(ts, 0)); d > ytNotifyMaxSkew || d < -ytNotifyMaxSkew {
		return errors.New("回调已过期")
	}
	ok, err := rsaVerifyBase64(buildSignContent(params), params["sign"], c.PlatformPublicKey)
	if err != nil {
		return fmt.Errorf("验签失败: %w", err)
	}
	if !ok {
		return errors.New("签名校验失败")
	}
	return nil
}

// ---------------- 签名工具 ----------------

// buildSignContent 待签名字符串：
// 过滤 sign/sign_type/空值 → 参数名 ASCII 升序 → k=v&k=v（值不编码）。
// 易支付与悦通云的这一步完全一致，差别只在末尾如何生成摘要。
//
// 注意必须忽略未知字段之外的一切多余处理：平台会新增回调字段，
// 验签要能容忍，所以是「对收到的全部非空字段」重建，而非按固定白名单。
func buildSignContent(params map[string]string) string {
	keys := make([]string, 0, len(params))
	for k, v := range params {
		if k == "sign" || k == "sign_type" || strings.TrimSpace(v) == "" {
			continue
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var sb strings.Builder
	for i, k := range keys {
		if i > 0 {
			sb.WriteByte('&')
		}
		sb.WriteString(k)
		sb.WriteByte('=')
		sb.WriteString(params[k])
	}
	return sb.String()
}

// buildYiPaySign 易支付 MD5：待签串末尾直接拼商户密钥再取 md5。
func buildYiPaySign(params map[string]string, key string) string {
	sum := md5.Sum([]byte(buildSignContent(params) + key))
	return hex.EncodeToString(sum[:])
}

func verifyYiPaySign(params map[string]string, key string) bool {
	got := params["sign"]
	if got == "" {
		return false
	}
	return strings.EqualFold(got, buildYiPaySign(params, key))
}

// parsePKCS8PrivateKey 解析裸 base64 的 PKCS#8 私钥（配置里不带 PEM 头尾）。
func parsePKCS8PrivateKey(b64 string) (*rsa.PrivateKey, error) {
	der, err := base64.StdEncoding.DecodeString(compactKey(b64))
	if err != nil {
		return nil, errors.New("商户私钥不是合法 base64")
	}
	k, err := x509.ParsePKCS8PrivateKey(der)
	if err != nil {
		return nil, errors.New("商户私钥格式错误（需 PKCS#8）")
	}
	rk, ok := k.(*rsa.PrivateKey)
	if !ok {
		return nil, errors.New("商户私钥不是 RSA 私钥")
	}
	return rk, nil
}

func parsePKIXPublicKey(b64 string) (*rsa.PublicKey, error) {
	der, err := base64.StdEncoding.DecodeString(compactKey(b64))
	if err != nil {
		return nil, errors.New("平台公钥不是合法 base64")
	}
	k, err := x509.ParsePKIXPublicKey(der)
	if err != nil {
		return nil, errors.New("平台公钥格式错误")
	}
	rk, ok := k.(*rsa.PublicKey)
	if !ok {
		return nil, errors.New("平台公钥不是 RSA 公钥")
	}
	return rk, nil
}

// compactKey 容忍从后台复制密钥时带上的 PEM 头尾与换行。
func compactKey(s string) string {
	r := strings.NewReplacer(
		"-----BEGIN PRIVATE KEY-----", "",
		"-----END PRIVATE KEY-----", "",
		"-----BEGIN PUBLIC KEY-----", "",
		"-----END PUBLIC KEY-----", "",
		"\r", "", "\n", "", " ", "", "\t", "",
	)
	return r.Replace(s)
}

func rsaSignBase64(content, privateKeyB64 string) (string, error) {
	key, err := parsePKCS8PrivateKey(privateKeyB64)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256([]byte(content))
	sig, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, sum[:])
	if err != nil {
		return "", fmt.Errorf("签名失败: %w", err)
	}
	return base64.StdEncoding.EncodeToString(sig), nil
}

func rsaVerifyBase64(content, signB64, publicKeyB64 string) (bool, error) {
	if signB64 == "" {
		return false, errors.New("签名为空")
	}
	key, err := parsePKIXPublicKey(publicKeyB64)
	if err != nil {
		return false, err
	}
	sig, err := base64.StdEncoding.DecodeString(signB64)
	if err != nil {
		return false, errors.New("签名不是合法 base64")
	}
	sum := sha256.Sum256([]byte(content))
	return rsa.VerifyPKCS1v15(key, crypto.SHA256, sum[:], sig) == nil, nil
}

// formatYuan 分 → 元，保留两位小数。
func formatYuan(cents int) string { return fmt.Sprintf("%.2f", float64(cents)/100) }

// buildQuery 构造跳转 URL 的 query（此处才对值做 url 编码）。
func buildQuery(params map[string]string) string {
	q := url.Values{}
	for k, v := range params {
		q.Set(k, v)
	}
	return q.Encode()
}
