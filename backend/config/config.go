package config

import (
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/viper"
)

type Config struct {
	Server     ServerConfig     `mapstructure:"server"`
	Database   DatabaseConfig   `mapstructure:"database"`
	Redis      RedisConfig      `mapstructure:"redis"`
	JWT        JWTConfig        `mapstructure:"jwt"`
	OSS        OSSConfig        `mapstructure:"oss"`
	OAuth      OAuthConfig      `mapstructure:"oauth"`
	YiPay      YiPayConfig      `mapstructure:"yipay"`
	LDC        LDCConfig        `mapstructure:"ldc"`
	YueTongYun YueTongYunConfig `mapstructure:"yuetongyun"`
	AntiCrawl  AntiCrawlConfig  `mapstructure:"anticrawl"`
	AuthLock   AuthLockConfig   `mapstructure:"auth_lock"`
}

// AntiCrawlConfig 反爬配置：限流阈值与可疑行为判定参数
type AntiCrawlConfig struct {
	Enable               bool     `mapstructure:"enable"`                 // 总开关
	GlobalPerMin         int      `mapstructure:"global_per_min"`         // 每用户全局限额（次/分钟）
	DataPerMin           int      `mapstructure:"data_per_min"`           // 数据类接口限额（次/分钟）
	AuthPerMin           int      `mapstructure:"auth_per_min"`           // 认证接口按 IP 限额（次/分钟）
	SuspectPerMin        int      `mapstructure:"suspect_per_min"`        // 被标记用户的降速限额（次/分钟）
	DistinctQuestions24h int      `mapstructure:"distinct_questions_24h"` // 24h 内访问不同题目数阈值
	SequentialThreshold  int      `mapstructure:"sequential_threshold"`   // 连续递增 ID 访问次数阈值
	AllowOrigins         []string `mapstructure:"allow_origins"`          // CORS 白名单，空则回退为 *
}

// AuthLockConfig 登录失败锁定：账号维度 + IP 维度，防撞库/爆破。
// 与 anticrawl.auth_per_min 互补——限流挡高频请求，锁定挡慢速持续试错。
type AuthLockConfig struct {
	Enable       bool `mapstructure:"enable"`         // 总开关；Redis 不可用时 fail-open
	MaxFailsUser int  `mapstructure:"max_fails_user"` // 同账号连续失败阈值
	MaxFailsIP   int  `mapstructure:"max_fails_ip"`   // 同 IP 连续失败阈值
	WindowSec    int  `mapstructure:"window_sec"`     // 计数窗口与锁定期（秒）
}

// YiPayConfig 易支付（彩虹易支付/码支付 通用接口）
type YiPayConfig struct {
	APIURL    string `mapstructure:"api_url"`    // 支付网关地址，以 / 结尾，如 https://pay.example.com/
	PID       string `mapstructure:"pid"`        // 商户ID
	Key       string `mapstructure:"key"`        // 商户密钥（用于签名）
	NotifyURL string `mapstructure:"notify_url"` // 异步通知地址（本站公网 https + /api/pay/notify）
	ReturnURL string `mapstructure:"return_url"` // 同步跳转地址（支付完成后浏览器返回的前端页面）
}

// LDCConfig LINUX DO Credit 积分支付。走其易支付兼容接口（MD5 签名），
// 与 YiPay 同构但密钥独立，因此回调也必须走独立路径分别验签。
type LDCConfig struct {
	APIURL    string `mapstructure:"api_url"`    // 网关基址，如 https://credit.linux.do/epay
	PID       string `mapstructure:"pid"`        // Client ID
	Key       string `mapstructure:"key"`        // Client Secret（用于签名）
	NotifyURL string `mapstructure:"notify_url"` // 异步通知地址（本站公网 https + /api/pay/ldc/notify）
	ReturnURL string `mapstructure:"return_url"` // 同步跳转地址（支付完成后浏览器返回的前端页面）
}

// YueTongYunConfig 悦通云支付。签名是 SHA256WithRSA + Base64（非对称），
// 因此配置的不是共享密钥，而是「商户私钥用于签名 / 平台公钥用于验签」两把不同的钥匙。
type YueTongYunConfig struct {
	APIURL             string `mapstructure:"api_url"`              // 网关基址，如 https://yuetongyun.cn
	PID                string `mapstructure:"pid"`                  // 商户ID
	MerchantPrivateKey string `mapstructure:"merchant_private_key"` // 商户私钥，PKCS#8 base64（不含 PEM 头尾）
	PlatformPublicKey  string `mapstructure:"platform_public_key"`  // 平台公钥，PKIX base64（不含 PEM 头尾），用于回调验签
	NotifyURL          string `mapstructure:"notify_url"`           // 异步通知地址（本站公网 https + /api/pay/ytpay/notify）
	ReturnURL          string `mapstructure:"return_url"`           // 同步跳转地址（支付完成后浏览器返回的前端页面）
}

type OAuthConfig struct {
	FrontendURL    string                `mapstructure:"frontend_url"`
	LinuxDo        LinuxDoOAuth          `mapstructure:"linuxdo"`
	FirstLoginBonus FirstLoginBonusConfig `mapstructure:"first_login_bonus"`
}

// FirstLoginBonusConfig LinuxDo OAuth 首次登录赠送会员的活动配置。
// 只对走 OAuth 新建账号的用户发放，老用户/邮箱合并用户不发，避免被刷。
type FirstLoginBonusConfig struct {
	Enable bool   `mapstructure:"enable"` // 总开关，关闭后首次登录不送
	Plan   string `mapstructure:"plan"`   // 套餐 key，复用 membership_plans.plan_key，默认 month
	Days   int    `mapstructure:"days"`   // 赠送天数，默认 30
	Title  string `mapstructure:"title"`  // 站内通知标题
}

type LinuxDoOAuth struct {
	ClientID          string `mapstructure:"client_id"`
	ClientSecret      string `mapstructure:"client_secret"`
	AuthorizeEndpoint string `mapstructure:"authorize_endpoint"`
	TokenEndpoint     string `mapstructure:"token_endpoint"`
	UserEndpoint      string `mapstructure:"user_endpoint"`
	RedirectURL       string `mapstructure:"redirect_url"`
}

type ServerConfig struct {
	Port int    `mapstructure:"port"`
	Mode string `mapstructure:"mode"`
	// SiteURL 站点对外地址（含协议，如 https://ruankao.example.com）。
	// sitemap 与 canonical 必须是绝对 URL，留空则不对外提供 sitemap。
	SiteURL string `mapstructure:"site_url"`
}

type DatabaseConfig struct {
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	User     string `mapstructure:"user"`
	Password string `mapstructure:"password"`
	DBName   string `mapstructure:"dbname"`
}

type RedisConfig struct {
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	Password string `mapstructure:"password"`
	DB       int    `mapstructure:"db"`
}

type JWTConfig struct {
	Secret          string `mapstructure:"secret"`
	ExpireHour      int    `mapstructure:"expire_hour"`
	GuestExpireHour int    `mapstructure:"guest_expire_hour"` // 游客令牌 TTL(小时),<=0 回落 24
}

// OSSConfig 对象存储（腾讯云 COS）。
// 密钥建议用只有指定存储桶写权限的子账号，不要用主账号密钥。
type OSSConfig struct {
	// Endpoint 存储桶访问域名（完整 URL）。留空则由 bucket + region 自动拼出，
	// 仅在私有云/特殊域名时才需要手填。
	Endpoint  string `mapstructure:"endpoint"`
	AccessKey string `mapstructure:"access_key"` // SecretId
	SecretKey string `mapstructure:"secret_key"` // SecretKey
	Bucket    string `mapstructure:"bucket"`     // 存储桶名，必须带 APPID，如 exam-1250000000
	Region    string `mapstructure:"region"`     // 地域，如 ap-guangzhou
	// BaseURL 返回给前端的 URL 前缀。配了 CDN/自定义域名就填它，
	// 留空则回落到存储桶默认域名。
	BaseURL string `mapstructure:"base_url"`
	Dir     string `mapstructure:"dir"`         // 对象键前缀目录，留空为 uploads
	MaxSize int    `mapstructure:"max_size_mb"` // 单张图片上限 MB，<=0 取默认 5
}

var AppConfig Config

// placeholderJWTSecret 仓库里长期存在的占位密钥。
// JWT secret 泄露 = 任何人可自签 role:1 的 token，因此 release 下必须拒绝启动。
const placeholderJWTSecret = "software-exam-secret-key-change-in-production"

// envPrefix 是环境变量前缀，yaml 里的层级用 _ 连接
// （SOFTWARE_EXAM_DATABASE_PASSWORD → database.password）。
// 测试里清理继承来的环境变量时也要用它，别再写第二份字面量。
const envPrefix = "SOFTWARE_EXAM"

func Load(path string) error {
	viper.SetConfigFile(path)
	viper.SetConfigType("yaml")

	// 环境变量优先于 yaml：SOFTWARE_EXAM_JWT_SECRET / _DATABASE_PASSWORD / _REDIS_PASSWORD 等
	// （yaml 里的层级用 _ 连接）。让凭据可以完全不进版本控制。
	viper.SetEnvPrefix(envPrefix)
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv()
	// AutomaticEnv 对 Unmarshal 中未在 yaml 出现的键不生效，敏感项显式绑定
	for _, key := range []string{
		"jwt.secret",
		"server.site_url",
		"database.password",
		"database.host",
		"database.user",
		"redis.password",
		"oauth.linuxdo.client_id",
		"oauth.linuxdo.client_secret",
		"yipay.pid",
		"yipay.key",
		"yipay.api_url",
		"ldc.pid",
		"ldc.key",
		"ldc.api_url",
		"yuetongyun.pid",
		"yuetongyun.api_url",
		"yuetongyun.merchant_private_key",
		"yuetongyun.platform_public_key",
		"oss.access_key",
		"oss.secret_key",
		"oss.bucket",
		"oss.region",
		"oss.base_url",
	} {
		_ = viper.BindEnv(key)
	}

	if err := viper.ReadInConfig(); err != nil {
		return err
	}
	// 同目录下的 config.local.yaml 是本机开发覆盖层，只需写与主配置不同的项。
	// 容器里没有这个文件，因此线上直接用主配置，不会被本机的库地址污染。
	if f, err := os.Open(localOverridePath(path)); err == nil {
		defer f.Close()
		if err := viper.MergeConfig(f); err != nil {
			return err
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	if err := viper.Unmarshal(&AppConfig); err != nil {
		return err
	}
	return validate()
}

func localOverridePath(path string) string {
	ext := filepath.Ext(path)
	return strings.TrimSuffix(path, ext) + ".local" + ext
}

// validate 配置兜底检查。debug 下不强校验，避免拦住本地开发。
func validate() error {
	if AppConfig.JWT.Secret == "" {
		return errors.New("jwt.secret 未配置")
	}
	if AppConfig.Server.Mode != "release" {
		return nil
	}
	if AppConfig.JWT.Secret == placeholderJWTSecret {
		return errors.New("jwt.secret 仍是仓库占位值，release 模式下拒绝启动")
	}
	if len(AppConfig.JWT.Secret) < 32 {
		return errors.New("jwt.secret 长度不足 32 字符")
	}
	return nil
}
