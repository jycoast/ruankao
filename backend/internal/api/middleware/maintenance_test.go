package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"software-exam/internal/application/service"
	"software-exam/internal/domain/model"

	"github.com/gin-gonic/gin"
)

// stubChecker 让中间件的测试不必搭数据库。
type stubChecker struct{ on bool }

func (s *stubChecker) MaintenanceEnabled(context.Context) bool { return s.on }

// maintenanceRouter 装一个最小的路由表，覆盖白名单内外的路径。
func maintenanceRouter(on bool, repo *fakeUserRepo) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(Maintenance(&stubChecker{on: on}, service.NewUserStateService(repo)))
	handler := func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"ok": true}) }
	r.GET("/api/health", handler)
	r.GET("/api/site-status", handler)
	r.GET("/api/subjects", handler)
	r.GET("/api/prerender/manifest", handler)
	r.GET("/api/admin/dashboard", handler)
	r.GET("/api/auth/login", handler)
	r.GET("/api/pay/notify", handler)
	r.GET("/api/questions", handler)
	return r
}

func hitWithToken(r *gin.Engine, path, token string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", path, nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	r.ServeHTTP(w, req)
	return w
}

// passed 判断请求是否真的穿过了维护中间件（而不是被挡下）。
// 不能只看 HTTP 状态：response.Error 恒返回 200，被挡的请求状态码同样是 200，
// 只有 body 里的业务码才分得清「放行」与「挡下」。
func passed(w *httptest.ResponseRecorder) bool {
	return w.Code == http.StatusOK && !strings.Contains(w.Body.String(), `"code":503`)
}

// 维护关闭时必须全部放行 —— 这是绝大多数请求走的那条路。
func TestMaintenanceOffPassesEverything(t *testing.T) {
	r := maintenanceRouter(false, &fakeUserRepo{})
	for _, p := range []string{"/api/questions", "/api/admin/dashboard", "/api/health"} {
		if w := hitWithToken(r, p, ""); !passed(w) {
			t.Fatalf("%s should pass when maintenance is off, got %d %s", p, w.Code, w.Body.String())
		}
	}
}

// 白名单里的路径在维护期间仍须放行，一条条验：
// health 挡了会被摘流、site-status 挡了前端不知道在维护、
// admin 挡了关不掉开关、login 挡了令牌一过期就永远进不去、
// prerender 挡了发版会被维护卡死、pay 挡了已支付订单永远不激活、
// subjects 挡了预渲染的 hub 页会静默失去科目分组。
func TestMaintenanceOnPassesWhitelist(t *testing.T) {
	r := maintenanceRouter(true, &fakeUserRepo{})
	for _, p := range []string{
		"/api/health",
		"/api/site-status",
		"/api/subjects",
		"/api/admin/dashboard",
		"/api/auth/login",
		"/api/prerender/manifest",
		"/api/pay/notify",
	} {
		if w := hitWithToken(r, p, ""); !passed(w) {
			t.Fatalf("whitelisted %s must pass during maintenance, got %d %s", p, w.Code, w.Body.String())
		}
	}
}

// 维护期间普通业务接口返回业务码 503。注意 HTTP 状态仍是 200 ——
// 这是仓库既有约定，前端拦截器按 body.code 分支。
func TestMaintenanceOnBlocksBusinessRoutes(t *testing.T) {
	r := maintenanceRouter(true, &fakeUserRepo{})
	w := hitWithToken(r, "/api/questions", "")
	if w.Code != http.StatusOK {
		t.Fatalf("http status must stay 200, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), `"code":503`) {
		t.Fatalf("expected business code 503, got %s", w.Body.String())
	}
}

// 维护响应必须带 data.maintenance 判别位与固定文案，缺一不可。
//
// 只断言 code 503 是不够的：503 不是维护专属 —— handler/upload.go 在对象存储
// 未配置时也回 503「图片上传未启用」。前端 api/index.ts 正是靠 data.maintenance
// 才区分得开；判别位丢了，一次正常的传图失败就会被硬弹到维护页。
// 业务码在 body 里才是判据（HTTP 恒为 200），所以这里断言的是 body 字面量。
func TestMaintenanceBlockBodyCarriesDiscriminator(t *testing.T) {
	r := maintenanceRouter(true, &fakeUserRepo{})
	body := hitWithToken(r, "/api/questions", "").Body.String()

	if !strings.Contains(body, `"code":503`) {
		t.Fatalf("precondition: 应返回业务码 503，实际 %s", body)
	}
	if !strings.Contains(body, `"data":{"maintenance":true}`) {
		t.Fatalf("维护响应必须带 data.maintenance=true 判别位，否则前端分不清维护与其它 503：%s", body)
	}
	if !strings.Contains(body, `"message":"系统维护中"`) {
		t.Fatalf("维护响应文案应为「系统维护中」，实际 %s", body)
	}
}

// c.Abort() 必须真的中止后续 handler —— 维护期间被挡下的请求绝不能继续走到
// 业务 handler，那些是写库的。探针 handler 会回 {"ok":true}，它一旦出现在 body 里
// 就说明 Abort 没生效。删掉 middleware.go 里的 c.Abort()，本用例立刻失败。
func TestMaintenanceBlockSkipsDownstreamHandler(t *testing.T) {
	r := maintenanceRouter(true, &fakeUserRepo{})
	body := hitWithToken(r, "/api/questions", "").Body.String()

	if !strings.Contains(body, `"code":503`) {
		t.Fatalf("precondition: 该路径应被维护挡下，实际 %s", body)
	}
	if strings.Contains(body, `"ok":true`) {
		t.Fatalf("下游 handler 在维护期间仍然跑了 —— c.Abort() 没生效：%s", body)
	}
}

// 管理员(role==1)必须能穿过维护 —— 否则开完维护自己也被锁在外面。
func TestMaintenanceOnPassesAdmin(t *testing.T) {
	u := &model.User{ID: 21, Username: "admin21", Role: 1, Status: 1}
	r := maintenanceRouter(true, &fakeUserRepo{user: u})
	if w := hitWithToken(r, "/api/questions", tokenFor(t, u)); !passed(w) {
		t.Fatalf("admin must pass during maintenance, got %d %s", w.Code, w.Body.String())
	}
}

// 普通登录用户(role==0)照挡。
func TestMaintenanceOnBlocksNormalUser(t *testing.T) {
	u := &model.User{ID: 22, Username: "u22", Role: 0, Status: 1}
	r := maintenanceRouter(true, &fakeUserRepo{user: u})
	w := hitWithToken(r, "/api/questions", tokenFor(t, u))
	if !strings.Contains(w.Body.String(), `"code":503`) {
		t.Fatalf("normal user must be blocked, got %s", w.Body.String())
	}
}

// 用户态服务读不到（账号在库里没了 → ErrUserNotFound）时解析不出管理员，
// 按普通用户处理返回 503。这不是 fail-open —— 用户态服务挂掉只让
// 「管理员这条捷径」失效，用户仍看到维护页，是安全的方向。
// 注意这与「开关仓库读不到」是两条不同的路径，后者在 SiteService 里 fail-open，见 Task 2。
func TestMaintenanceBlocksWhenUserStateUnreadable(t *testing.T) {
	u := &model.User{ID: 23, Username: "admin23", Role: 1, Status: 1}
	tok := tokenFor(t, u)
	// repo 里没有这个用户 —— 模拟用户态服务查不到
	r := maintenanceRouter(true, &fakeUserRepo{})
	w := hitWithToken(r, "/api/questions", tok)
	if !strings.Contains(w.Body.String(), `"code":503`) {
		t.Fatalf("unreadable user state must fall back to blocking, got %s", w.Body.String())
	}
}

// 游客 token(role 2)没有 DB 行，解析不出身份 → 挡。
func TestMaintenanceBlocksGuest(t *testing.T) {
	u := &model.User{ID: 0, Username: "guest", Role: 2, Status: 1}
	r := maintenanceRouter(true, &fakeUserRepo{})
	w := hitWithToken(r, "/api/questions", tokenFor(t, u))
	if !strings.Contains(w.Body.String(), `"code":503`) {
		t.Fatalf("guest must be blocked, got %s", w.Body.String())
	}
}
