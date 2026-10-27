package middleware

import (
	"context"
	"errors"
	"log"
	"net/http"
	"strings"

	"software-exam/config"
	"software-exam/internal/application/service"
	"software-exam/pkg/auth"
	"software-exam/pkg/online"
	"software-exam/pkg/response"

	"github.com/gin-gonic/gin"
)

// AuthMiddleware 校验 token，并以数据库中的 status/role 为准。
//
// token 一经签发 72 小时内无法撤销，只信 token 里的 role 会让封号与降权在过期前形同虚设。
// 因此每个请求都查一次实时状态（UserStateService 内部 Redis 缓存 60s）。
func AuthMiddleware(userState *service.UserStateService) gin.HandlerFunc {
	return func(c *gin.Context) {
		tokenString := c.GetHeader("Authorization")
		if tokenString == "" {
			response.Unauthorized(c, "请先登录")
			c.Abort()
			return
		}

		tokenString = strings.TrimPrefix(tokenString, "Bearer ")
		claims, err := auth.ParseToken(tokenString)
		if err != nil {
			response.Unauthorized(c, "登录已过期，请重新登录")
			c.Abort()
			return
		}

		// 虚拟游客(role 2):无 DB 行,不回读实时状态、不做封禁校验、不打在线心跳。
		// 只读强制:非安全方法统一 403——一条规则覆盖全部写端点,无需逐路由挂守卫。
		if claims.Role == auth.RoleGuest {
			c.Set("user_id", uint64(0))
			c.Set("username", auth.GuestUsername)
			c.Set("role", auth.RoleGuest)
			switch c.Request.Method {
			case http.MethodGet, http.MethodHead, http.MethodOptions:
				c.Next()
			default:
				response.Error(c, 403, "游客模式仅支持浏览，请登录后使用完整功能")
				c.Abort()
			}
			return
		}

		st, err := userState.Get(c.Request.Context(), claims.UserID)
		if err != nil {
			if errors.Is(err, service.ErrUserNotFound) {
				response.Unauthorized(c, "账号不存在，请重新登录")
			} else {
				// 数据库故障时 fail-closed：宁可全站不可用，也不放过被封账号
				log.Printf("[auth] load user state %d failed: %v", claims.UserID, err)
				response.ServerError(c, "服务暂时不可用，请稍后重试")
			}
			c.Abort()
			return
		}
		if st.Banned() {
			// 用 401 而非 403：前端据此清 token 并跳登录，
			// 403 只会弹个 toast，被封用户仍停留在已登录界面反复报错。
			response.Unauthorized(c, "账号已被禁用")
			c.Abort()
			return
		}

		c.Set("user_id", claims.UserID)
		c.Set("username", claims.Username)
		c.Set("role", st.Role)
		// 在线统计心跳：Redis ZSET 记最近活跃时间，失败静默不影响请求
		online.Heartbeat(c.Request.Context(), claims.UserID)
		c.Next()
	}
}

// identity 请求者身份。ok=false 表示「未识别」，调用方按匿名处理。
type identity struct {
	UserID   uint64
	Username string
	Role     int8
}

// resolveIdentity 从 Authorization 头解析请求者身份，不强制登录。
//
// 无 token / token 无效 / 用户态读不到 / 已被封禁一律返回 ok=false ——
// 公开内容本就人人可读，在这里 fail-closed 只会让公开页跟着数据库一起挂。
//
// 抽成公共函数是因为 OptionalAuth 与 Maintenance 都要做同一件事：
// 前者据此写入 context，后者据此判断是不是管理员。两份实现迟早会漂移。
func resolveIdentity(c *gin.Context, userState *service.UserStateService) (identity, bool) {
	tokenString := strings.TrimPrefix(c.GetHeader("Authorization"), "Bearer ")
	if tokenString == "" {
		return identity{}, false
	}
	claims, err := auth.ParseToken(tokenString)
	if err != nil {
		return identity{}, false
	}
	// 虚拟游客(role 2)没有 DB 行，历史上的 OptionalAuth 就把它降级成匿名
	// （userState.Get(0) 必然 not found）。这里保持同一结果 ——
	// 公开页上的游客分支行为不该被这次重构悄悄改掉，维护中间件也不受影响
	// （游客不是管理员，本来就要挡）。
	if claims.Role == auth.RoleGuest {
		return identity{}, false
	}
	st, err := userState.Get(c.Request.Context(), claims.UserID)
	if err != nil || st.Banned() {
		return identity{}, false
	}
	return identity{UserID: claims.UserID, Username: claims.Username, Role: st.Role}, true
}

// OptionalAuth 解析 token 但不强制登录：带有效 token 就把身份写进 context，
// 没带或已失效则以匿名身份继续。供知识点/范文这类对搜索引擎开放的内容路由使用 ——
// 这些页面必须能被未登录的爬虫和访客读到，否则改 history 路由、出 sitemap 都是白做。
func OptionalAuth(userState *service.UserStateService) gin.HandlerFunc {
	return func(c *gin.Context) {
		if id, ok := resolveIdentity(c, userState); ok {
			c.Set("user_id", id.UserID)
			c.Set("username", id.Username)
			c.Set("role", id.Role)
		}
		c.Next()
	}
}

func AdminMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		role, exists := c.Get("role")
		if !exists || role.(int8) != 1 {
			response.Forbidden(c, "无权限访问")
			c.Abort()
			return
		}
		c.Next()
	}
}

// RequireMembership 会员专属功能网关。须在 AuthMiddleware 之后使用。
// 管理员(role==1)直接放行；非会员返回 402，前端据此引导开通。
func RequireMembership(membershipSvc *service.MembershipService) gin.HandlerFunc {
	return func(c *gin.Context) {
		if role, ok := c.Get("role"); ok {
			if r, ok := role.(int8); ok && r == 1 {
				c.Next()
				return
			}
		}
		userID := c.GetUint64("user_id")
		if !membershipSvc.IsActive(userID) {
			response.Error(c, 402, "该功能为会员专属，请先开通会员")
			c.Abort()
			return
		}
		c.Next()
	}
}

// AIQuota 非会员 AI 免费试用网关。须在 AuthMiddleware 之后使用。
// 管理员与会员无限制；非会员在免费额度内放行并计数，用尽返回 402 引导开通。
// 用户配了自己的 AI（自带 key）时不占用平台额度。
func AIQuota(membershipSvc *service.MembershipService, aiSvc *service.AIService) gin.HandlerFunc {
	return func(c *gin.Context) {
		if role, ok := c.Get("role"); ok {
			if r, ok := role.(int8); ok {
				if r == auth.RoleGuest {
					// 游客不消耗免费试用额度,也不走会员判断
					response.Error(c, 403, "AI 功能需登录后使用")
					c.Abort()
					return
				}
				if r == 1 {
					c.Next()
					return
				}
			}
		}
		userID := c.GetUint64("user_id")
		if membershipSvc.IsActive(userID) || aiSvc.UsesOwnConfig(userID) {
			c.Next()
			return
		}

		// 先占额度再调上游。原实现「查→判→响应结束后再扣」在并发下会全部通过检查，
		// 免费次数形同虚设。失败时通过 defer 归还，保持「失败不扣次数」的原有语义。
		ok, limit, err := aiSvc.ReserveTrial(userID)
		if err != nil {
			// 额度仓储故障不能放行，否则等于绕过付费墙
			response.Error(c, 500, "额度校验失败，请稍后重试")
			c.Abort()
			return
		}
		if !ok {
			if limit == 0 {
				response.Error(c, 402, "该功能为会员专属，请先开通会员")
			} else {
				response.Error(c, 402, "免费试用次数已用完，开通会员可无限使用")
			}
			c.Abort()
			return
		}

		consumed := true
		defer func() {
			// 兜底：handler panic 时 defer 仍会执行，不让额度被白吞
			if consumed {
				aiSvc.RefundTrial(userID)
			}
		}()

		c.Next()
		// 失败不扣次数。响应体恒为 HTTP 200，靠 response.Error 与 SSE handler 打的标记判断。
		if c.Writer.Status() < 400 && !c.GetBool(response.BizErrorKey) && !c.GetBool("ai_failed") {
			consumed = false // 成功：额度留下
		}
	}
}

// CorsMiddleware 按配置白名单放行来源。
// allow_origins 为空时：debug 模式回退为 *（本地开发方便），其它模式一律不发 CORS 头，
// 避免生产漏配白名单就变成任意站点可跨域携带请求。
func CorsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		allowed := config.AppConfig.AntiCrawl.AllowOrigins
		origin := c.GetHeader("Origin")
		if len(allowed) == 0 {
			if config.AppConfig.Server.Mode == "debug" {
				c.Header("Access-Control-Allow-Origin", "*")
			}
		} else if origin != "" {
			for _, o := range allowed {
				if o == origin {
					c.Header("Access-Control-Allow-Origin", origin)
					c.Header("Access-Control-Allow-Credentials", "true")
					c.Header("Vary", "Origin")
					break
				}
			}
		}
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Origin, Content-Type, Authorization")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	}
}

// MaintenanceChecker 维护开关的读取口。
// 声明成接口而不是直接依赖 *service.SiteService，是为了让中间件的测试
// 塞一个假 checker 就行，不必搭数据库。
type MaintenanceChecker interface {
	MaintenanceEnabled(ctx context.Context) bool
}

// maintenanceWhitelist 维护期间仍须放行的路径前缀。
// 每一条都有非它不可的理由，删任何一条之前先想清楚后果（见各自的注释）。
//
// 新增**构建期脚本或服务器对服务器**的消费者时，记得回来加一条：它们不经过
// 浏览器，没有人会看见维护页，只会静默拿到一个 {code:503} 当成成功。审查里
// 找到的两个缺口（prerender 的 /api/subjects、支付回调）都是这么来的。
var maintenanceWhitelist = []string{
	"/api/health",      // 容器探活/负载均衡摘流：维护中的实例也得是健康的，否则被摘掉就没人提供维护页了
	"/api/site-status", // 前端靠它才知道在维护
	"/api/subjects",    // 同上一条构建链：npm run build 的预渲染除 manifest 外还要拉科目名给 hub 页分组。
	// 挡了它 fetchSubjects 会静默拿到空 map（HTTP 200 + code 503，res.ok 为真、不抛错），
	// /knowledge 与 /essays 两个 hub 页把全部条目归进「其它」照常发布。它不是新增暴露面：
	// router/exam.go 上本来就是 OptionalAuth 的公开只读路由。
	"/api/admin",              // 否则管理员进不去后台关开关
	"/api/auth/login",         // 否则管理员令牌一过期就永远进不来
	"/api/prerender/manifest", // 构建期预渲染要能跑，别让维护卡死发版
	"/api/pay/",               // 三条支付回调全是服务器对服务器，用户浏览器访问不到，挡它没有安全收益。
	// 而回调契约是「HTTP 200 + body success」才算投递成功，response.Error 却恒返回 HTTP 200，
	// 网关若只看状态码就会当成已投递、订单永不激活 —— 钱收了会员不开，只能人工补。
}

// Maintenance 系统维护中间件。全局注册，必须在 CorsMiddleware 之后 ——
// CORS 预检不带 Authorization，被维护挡下的话浏览器只会报跨域失败，
// 而不是给用户一个维护提示。
//
// 开销做成阶梯式，维护关闭时（绝大多数时间）只花一次原子读：
//  1. 开关为关 → 放行
//  2. 路径在白名单里 → 放行
//  3. role==1（管理员）→ 放行
//  4. 否则 → 业务码 503
func Maintenance(checker MaintenanceChecker, userState *service.UserStateService) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !checker.MaintenanceEnabled(c.Request.Context()) {
			c.Next()
			return
		}

		path := c.Request.URL.Path
		for _, prefix := range maintenanceWhitelist {
			if strings.HasPrefix(path, prefix) {
				c.Next()
				return
			}
		}

		// role 解析只在这里才发生（即维护已开启、且路径不在白名单里），
		// 所以平时不会与路由组的 OptionalAuth 重复查用户态。
		if id, ok := resolveIdentity(c, userState); ok && id.Role == 1 {
			c.Next()
			return
		}

		// HTTP 状态仍是 200，业务码才是 503 —— 仓库既有约定，
		// 前端 axios 拦截器按 body.code 分支。
		// data.maintenance 是判别位：503 不是维护专属（handler/upload.go 在对象存储
		// 未配置时也回 503「图片上传未启用」），前端只凭 code 分支会把一次正常的
		// 传图失败硬弹到维护页。前端要求 code===503 && data.maintenance===true 才跳转。
		response.ErrorWithData(c, 503, "系统维护中", gin.H{"maintenance": true})
		c.Abort()
	}
}
