package middleware

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"software-exam/config"
	"software-exam/pkg/redisclient"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

// —— 限流：Redis 固定窗口（Lua 原子 INCR+EXPIRE），按用户/IP 分键 ——
//
// Key 布局：
//   rl:u:{user_id}:{scope}   已登录用户，scope 区分 global/data
//   rl:ip:{ip}:{scope}       未登录接口（如 /api/auth/*）
//   as:suspect:{user_id}     行为检测标记（见 antiscrape.go），存在则切换为降速限额

// rateLimitScript 原子地自增计数并在首个请求时设置窗口过期。
// 返回当前窗口内的计数值。
var rateLimitScript = redis.NewScript(`
local current = redis.call('INCR', KEYS[1])
if current == 1 then
    redis.call('EXPIRE', KEYS[1], ARGV[1])
end
return current
`)

const rateLimitWindow = time.Minute

// acConfig 读取反爬配置并填充默认值，保证配置缺省时行为合理。
func acConfig() config.AntiCrawlConfig {
	cfg := config.AppConfig.AntiCrawl
	if cfg.GlobalPerMin <= 0 {
		cfg.GlobalPerMin = 120
	}
	if cfg.DataPerMin <= 0 {
		cfg.DataPerMin = 30
	}
	if cfg.AuthPerMin <= 0 {
		cfg.AuthPerMin = 10
	}
	if cfg.SuspectPerMin <= 0 {
		cfg.SuspectPerMin = 10
	}
	if cfg.AIPerMin <= 0 {
		cfg.AIPerMin = 10 // 生成类调用耗时长、成本高，一次/6 秒已够正常使用
	}
	if cfg.AIMaxConcurrent <= 0 {
		cfg.AIMaxConcurrent = 2
	}
	if cfg.DistinctQuestions24h <= 0 {
		cfg.DistinctQuestions24h = 500
	}
	if cfg.SequentialThreshold <= 0 {
		cfg.SequentialThreshold = 45 // 最近 50 次中递增占比 90%
	}
	return cfg
}

// allow 在 Redis 上执行一次窗口计数。Redis 不可用时 fail-open 放行。
func allow(c *gin.Context, key string, limit int) bool {
	rdb := redisclient.Client
	if rdb == nil {
		return true
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 500*time.Millisecond)
	defer cancel()
	count, err := rateLimitScript.Run(ctx, rdb, []string{key}, int(rateLimitWindow.Seconds())).Int64()
	if err != nil {
		return true // fail-open：限流组件故障不影响业务
	}
	return count <= int64(limit)
}

func rejectTooMany(c *gin.Context) {
	c.Header("Retry-After", "60")
	c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
		"code":    429,
		"message": "操作过于频繁，请稍后再试",
	})
}

// isSuspect 查询用户是否被行为检测标记（标记后限额自动降速）。
func isSuspect(c *gin.Context, userID uint64) bool {
	rdb := redisclient.Client
	if rdb == nil || userID == 0 {
		return false
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 300*time.Millisecond)
	defer cancel()
	n, err := rdb.Exists(ctx, fmt.Sprintf("as:suspect:%d", userID)).Result()
	return err == nil && n > 0
}

// guestPublicKey 虚拟游客计费键：与匿名公开内容同桶、同额度。
// 游客没有独立账号，若按 rl:u:0 计，全站游客共用一个桶互相饿死；退到 IP 桶才能
// 既限制单机批量抓库又不误伤同出口的正常用户。key 布局见文件头注释。
func guestPublicKey(ip string) string {
	return fmt.Sprintf("rl:ip:%s:public", ip)
}

// RateLimitByUser 按登录用户限流。scope 区分不同额度池（global/data）。
// 须挂在 AuthMiddleware 之后。被 antiscrape 标记的用户自动降速。
func RateLimitByUser(scope string, limitFn func(cfg config.AntiCrawlConfig) int) gin.HandlerFunc {
	return func(c *gin.Context) {
		cfg := acConfig()
		if !cfg.Enable {
			c.Next()
			return
		}
		userID := c.GetUint64("user_id")
		if userID == 0 {
			// 虚拟游客(user_id=0)是唯一会带 Auth 以 uid=0 到达此处的身份:
			// 不再直接放行(零限流=抓库漏洞),退化为按 IP 公开桶计费,额度同匿名档 data*3
			if !allow(c, guestPublicKey(c.ClientIP()), cfg.DataPerMin*3) {
				rejectTooMany(c)
				return
			}
			c.Next()
			return
		}
		limit := limitFn(cfg)
		if isSuspect(c, userID) && cfg.SuspectPerMin < limit {
			limit = cfg.SuspectPerMin
		}
		if !allow(c, fmt.Sprintf("rl:u:%d:%s", userID, scope), limit) {
			rejectTooMany(c)
			return
		}
		c.Next()
	}
}

// RateLimitGlobal 每用户全局限额。
func RateLimitGlobal() gin.HandlerFunc {
	return RateLimitByUser("global", func(cfg config.AntiCrawlConfig) int { return cfg.GlobalPerMin })
}

// RateLimitData 数据类接口（题库/试卷/知识点）限额，比全局更严格。
func RateLimitData() gin.HandlerFunc {
	return RateLimitByUser("data", func(cfg config.AntiCrawlConfig) int { return cfg.DataPerMin })
}

// RateLimitPublic 公开内容接口（知识点/范文）限流。
//
// 未登录访客无 user_id,若走 RateLimitByUser 会退化成零限流;
// 这些路由为了 SEO 必须允许匿名读取,因此匿名与游客请求都改按 IP 计数(guestPublicKey)。
// 额度取 DataPerMin 的三倍:同一出口 IP 后面可能是整间办公室或校园网,
// 按单用户额度卡会误伤,但仍能挡住单机批量抓取。
func RateLimitPublic() gin.HandlerFunc {
	return func(c *gin.Context) {
		cfg := acConfig()
		if !cfg.Enable {
			c.Next()
			return
		}
		if userID := c.GetUint64("user_id"); userID != 0 {
			limit := cfg.DataPerMin
			if isSuspect(c, userID) && cfg.SuspectPerMin < limit {
				limit = cfg.SuspectPerMin
			}
			if !allow(c, fmt.Sprintf("rl:u:%d:data", userID), limit) {
				rejectTooMany(c)
				return
			}
			c.Next()
			return
		}
		if !allow(c, guestPublicKey(c.ClientIP()), cfg.DataPerMin*3) {
			rejectTooMany(c)
			return
		}
		c.Next()
	}
}

// RateLimitByIP 按客户端 IP 限流，用于登录/注册等未认证接口，
// 防止批量注册小号绕过按用户限流。
func RateLimitByIP() gin.HandlerFunc {
	return func(c *gin.Context) {
		cfg := acConfig()
		if !cfg.Enable {
			c.Next()
			return
		}
		if !allow(c, fmt.Sprintf("rl:ip:%s:auth", c.ClientIP()), cfg.AuthPerMin) {
			rejectTooMany(c)
			return
		}
		c.Next()
	}
}

// RateLimitAction 敏感动作限流：用户维度和 IP 维度同时计数，两者任一超限即拒绝。
// 只按用户限流挡不住「注册一批小号轮流试卡密」，只按 IP 限流又会误伤同出口的正常用户。
//
// 与 RateLimitByUser 不同，这里不受 anticrawl.enable 开关影响 ——
// 反爬是可选功能，而卡密爆破防护任何时候都不该被关掉。
func RateLimitAction(scope string, perMin int) gin.HandlerFunc {
	if perMin <= 0 {
		perMin = 10
	}
	return func(c *gin.Context) {
		if userID := c.GetUint64("user_id"); userID != 0 {
			if !allow(c, fmt.Sprintf("rl:act:u:%d:%s", userID, scope), perMin) {
				rejectTooMany(c)
				return
			}
		}
		if !allow(c, fmt.Sprintf("rl:act:ip:%s:%s", c.ClientIP(), scope), perMin*3) {
			rejectTooMany(c)
			return
		}
		c.Next()
	}
}

// RateLimitAI AI 生成接口限额：用户 + IP 双桶，且不受 anticrawl.enable 影响。
// 与 RateLimitAction 同理——反爬是可选的，AI 调用成本护栏不是。
func RateLimitAI() gin.HandlerFunc {
	return func(c *gin.Context) {
		limit := acConfig().AIPerMin
		if userID := c.GetUint64("user_id"); userID != 0 {
			if !allow(c, fmt.Sprintf("rl:ai:u:%d", userID), limit) {
				rejectTooMany(c)
				return
			}
		}
		// 同一出口 IP 可能是办公室/校园网，额度放宽到 3 倍，只挡单机刷量
		if !allow(c, fmt.Sprintf("rl:ai:ip:%s", c.ClientIP()), limit*3) {
			rejectTooMany(c)
			return
		}
		c.Next()
	}
}

// —— AI 在途请求计数：限制单用户同时进行的生成请求数 ——

const aiInflightTTL = 10 * time.Minute

var aiInflightAcquire = redis.NewScript(`
local current = redis.call('INCR', KEYS[1])
if current == 1 then
    redis.call('EXPIRE', KEYS[1], ARGV[1])
end
return current
`)

// 返回 0 时删除键，避免计数长期挂在 0 上
var aiInflightRelease = redis.NewScript(`
local current = redis.call('DECR', KEYS[1])
if current <= 0 then
    redis.call('DEL', KEYS[1])
    return 0
end
return current
`)

// AIConcurrency 限制单个用户同时在途的 AI 生成请求数。
// 只有分钟级限流挡不住「一分钟内同时开 N 条 SSE 长连接」；在途计数用 Redis，
// 多实例部署下同样有效，进程崩溃由 TTL 自动回收。Redis 不可用时 fail-open。
func AIConcurrency() gin.HandlerFunc {
	return func(c *gin.Context) {
		rdb := redisclient.Client
		if rdb == nil {
			c.Next()
			return
		}
		limit := acConfig().AIMaxConcurrent
		key := fmt.Sprintf("rl:ai:fly:ip:%s", c.ClientIP())
		if userID := c.GetUint64("user_id"); userID != 0 {
			key = fmt.Sprintf("rl:ai:fly:u:%d", userID)
		}

		ctx, cancel := context.WithTimeout(c.Request.Context(), 500*time.Millisecond)
		count, err := aiInflightAcquire.Run(ctx, rdb, []string{key}, int(aiInflightTTL.Seconds())).Int64()
		cancel()
		if err != nil {
			c.Next() // fail-open：限流组件故障不影响业务
			return
		}
		release := func() {
			// 用独立 context：客户端断开后 request context 已取消，释放仍要执行
			rctx, rcancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer rcancel()
			_, _ = aiInflightRelease.Run(rctx, rdb, []string{key}).Result()
		}
		if count > int64(limit) {
			release()
			rejectTooMany(c)
			return
		}
		defer release()
		c.Next()
	}
}
