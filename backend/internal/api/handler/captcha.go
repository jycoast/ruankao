package handler

import (
	"context"
	"strings"
	"time"

	"software-exam/pkg/response"

	"software-exam/pkg/redisclient"

	"github.com/gin-gonic/gin"
	"github.com/mojocn/base64Captcha"
	"github.com/redis/go-redis/v9"
)

const captchaExpire = 5 * time.Minute

// redisCaptchaStore 实现 base64Captcha.Store，答案存 Redis。
// 一次性使用（Verify / Get(clear=true) 读后即删）；TTL 5 分钟兜底过期。
type redisCaptchaStore struct{}

func (s *redisCaptchaStore) Set(id, value string) {
	rdb := redisclient.Client
	if rdb == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	_ = rdb.Set(ctx, "captcha:"+id, value, captchaExpire).Err()
}

// Get 返回答案，base64Captcha.Store 接口必需。clear=true 时读后即删：
// 用原子 GetDel，避免并发重复消费同一验证码；clear=false 时仅读取。
// Redis 故障时返回 ""，fail-open 由 Verify 兜底。
func (s *redisCaptchaStore) Get(id string, clear bool) string {
	rdb := redisclient.Client
	if rdb == nil {
		return ""
	}
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	key := "captcha:" + id
	if clear {
		val, err := rdb.GetDel(ctx, key).Result()
		if err != nil {
			return ""
		}
		return val
	}
	val, err := rdb.Get(ctx, key).Result()
	if err != nil {
		return ""
	}
	return val
}

// captchaAnswerEqual 大小写不敏感比较，且空答案恒不匹配（id 无效/已过期/已用完）。
func captchaAnswerEqual(stored, answer string) bool {
	return stored != "" && strings.EqualFold(stored, answer)
}

// verifyWithGetResult 纯决策：err==redis.Nil 为真「无验证码」→ false（验证码不存在/已消费/已过期）；
// 其余错误（Redis 故障）→ fail-open true，宁可放行也不锁死注册；
// 否则按答案大小写不敏感比较。
func verifyWithGetResult(val string, err error, answer string) bool {
	if err == redis.Nil {
		return false
	}
	if err != nil {
		return true
	}
	return captchaAnswerEqual(val, answer)
}

// Verify 大小写不敏感，一次性（读后即删）。GetDel 原子消费：
// 无论答案对错都删除，保留防爆破的一次性语义。
// 区分「无验证码」（redis.Nil → false）与「Redis 故障」（其余错误 → fail-open true，
// 与限流中间件 allow() 同一策略）。
func (s *redisCaptchaStore) Verify(id, answer string, clear bool) bool {
	rdb := redisclient.Client
	if rdb == nil {
		return true
	}
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	val, err := rdb.GetDel(ctx, "captcha:"+id).Result()
	return verifyWithGetResult(val, err, answer)
}

var captchaInstance = base64Captcha.NewCaptcha(
	base64Captcha.NewDriverDigit(80, 240, 4, 0.7, 80),
	&redisCaptchaStore{},
)

// GetCaptcha GET /api/auth/captcha —— 下发注册图形验证码。
func GetCaptcha(c *gin.Context) {
	id, b64s, err := captchaInstance.Generate()
	if err != nil {
		response.ServerError(c, "验证码生成失败")
		return
	}
	response.Success(c, gin.H{"captcha_id": id, "image_base64": b64s})
}

// VerifyCaptcha 校验注册验证码（一次性）。Redis 故障（含 nil client）时 fail-open 放行。
// 注意：必须走 Store.Verify 而不是 Captcha.Verify —— 库的 Captcha.Verify
// 直接调 Store.Get 且大小写敏感，会绕过 redisCaptchaStore.Verify 的 fail-open 逻辑。
func VerifyCaptcha(id, code string) bool {
	return captchaInstance.Store.Verify(id, code, true)
}
