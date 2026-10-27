package router

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"software-exam/cmd/server/wire"
	"software-exam/config"
	"software-exam/internal/api/handler"
	"software-exam/internal/api/middleware"
	"software-exam/internal/application/service"
	"software-exam/internal/domain/model"
	"software-exam/internal/domain/repository"
	"software-exam/pkg/auth"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// stubSiteRepo 内存版站点开关仓储。本文件验的是「装起来的这一整套跑不跑得通」，
// 不是仓储本身（仓储/服务的用例在各自包里），所以不起库。
type stubSiteRepo struct{ on bool }

func (s *stubSiteRepo) MaintenanceEnabled() (bool, error)    { return s.on, nil }
func (s *stubSiteRepo) UpdateMaintenance(enabled bool) error { s.on = enabled; return nil }

// stubUserRepo 只实现鉴权真正会调的 FindByID，其余方法置 panic
// （与 middleware 包 auth_test.go 的 fakeUserRepo 同款）。
type stubUserRepo struct{ user *model.User }

func (f *stubUserRepo) FindByID(id uint64) (*model.User, error) {
	if f.user == nil || f.user.ID != id {
		return nil, gorm.ErrRecordNotFound
	}
	cp := *f.user
	return &cp, nil
}

func (f *stubUserRepo) Create(*model.User) error                                 { panic("unused") }
func (f *stubUserRepo) FindByIDs([]uint64) ([]model.User, error)                 { panic("unused") }
func (f *stubUserRepo) FindByUsername(string) (*model.User, error)               { panic("unused") }
func (f *stubUserRepo) FindByEmail(string) (*model.User, error)                  { panic("unused") }
func (f *stubUserRepo) FindByLinuxDoID(uint64) (*model.User, error)              { panic("unused") }
func (f *stubUserRepo) Update(*model.User) error                                 { panic("unused") }
func (f *stubUserRepo) UpdateSubject(uint64, uint64) error                       { panic("unused") }
func (f *stubUserRepo) List(repository.UserListReq) ([]model.User, int64, error) { panic("unused") }

func passThrough(c *gin.Context) { c.Next() }

// wiredRouter 走真正的 NewRouter，只把「会真连库/连 Redis 的那几项」换成内存桩。
// 其余 handler 留 nil：gin 的方法值只在请求真的走到时才解引用，
// 下面每个用例命中的路径都在到达它们之前就结束（被维护挡下 / 被鉴权拦下）。
func wiredRouter(t *testing.T, maintenance bool, user *model.User) (*gin.Engine, *stubSiteRepo) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	// gin.Default() 的访问日志会把测试输出淹掉
	gin.DefaultWriter = io.Discard
	config.AppConfig.JWT.Secret = "router-test-secret-at-least-32-chars!!!!"
	config.AppConfig.JWT.ExpireHour = 72

	repo := &stubSiteRepo{on: maintenance}
	site := service.NewSiteService(repo)
	userState := service.NewUserStateService(&stubUserRepo{user: user})

	app := &wire.App{
		Repos:    &wire.Repos{},
		Handlers: &wire.Handlers{Site: handler.NewSiteHandler(site)},
		Middlewares: &wire.Middlewares{
			Cors:        middleware.CorsMiddleware(),
			Maintenance: middleware.Maintenance(site, userState),
			Auth:        middleware.AuthMiddleware(userState),
			Admin:       middleware.AdminMiddleware(),
			// 限流器在「注册路由时」就被实例化（见 auth/comment/membership 路由），
			// 留 nil 会让 NewRouter 当场 panic。
			RateAction:    func(string, int) gin.HandlerFunc { return passThrough },
			RateAI:        passThrough,
			AIConcurrency: passThrough,
			RateGlobal:    passThrough,
			RateData:      passThrough,
			RatePublic:    passThrough,
			RateIP:        passThrough,
			OptionalAuth:  func(c *gin.Context) { c.Next() },
		},
	}
	return NewRouter(app), repo
}

func hit(r *gin.Engine, method, path, token string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	req := httptest.NewRequest(method, path, nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	r.ServeHTTP(w, req)
	return w
}

func adminToken(t *testing.T, u *model.User) string {
	t.Helper()
	tok, err := auth.GenerateToken(u.ID, u.Username, u.Role)
	if err != nil {
		t.Fatal(err)
	}
	return tok
}

// 前端守卫轮询的就是这个字面量路径，维护白名单里写的也是同一条字符串。
// 注册到别处（换前缀、多一截、改个名）前端永远不知道站点在维护，
// 维护页对谁都不会出现 —— 所以直接对路由表断言字面量。
func TestSiteRoutesRegisteredAtLiteralPaths(t *testing.T) {
	r, _ := wiredRouter(t, false, nil)
	registered := map[string]bool{}
	for _, rt := range r.Routes() {
		registered[rt.Method+" "+rt.Path] = true
	}
	for _, want := range []string{
		"GET /api/site-status",
		"GET /api/admin/site-settings",
		"PUT /api/admin/site-settings",
	} {
		if !registered[want] {
			t.Fatalf("路由 %s 未注册", want)
		}
	}
}

// 开关关着时也要能答（前端每次启动都问一次），答 false。
func TestSiteStatusReportsOff(t *testing.T) {
	r, _ := wiredRouter(t, false, nil)
	w := hit(r, http.MethodGet, "/api/site-status", "")
	if w.Code != http.StatusOK {
		t.Fatalf("HTTP 状态应为 200，实际 %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), `"maintenance":false`) {
		t.Fatalf("期望 maintenance=false，实际 %s", w.Body.String())
	}
}

// 维护期间这个接口必须给出真实答案，而不是被自己挡成 503 ——
// 它一被挡，前端只会走通用错误分支，维护页永远出不来。
func TestSiteStatusReportsOnDuringMaintenance(t *testing.T) {
	r, _ := wiredRouter(t, true, nil)
	w := hit(r, http.MethodGet, "/api/site-status", "")
	if !strings.Contains(w.Body.String(), `"maintenance":true`) {
		t.Fatalf("维护期间 /api/site-status 应返回 maintenance=true，实际 %s", w.Body.String())
	}
}

// 普通业务接口在维护期间按仓库约定返回业务码 503，HTTP 状态仍是 200。
func TestMaintenanceBlocksBusinessRoute(t *testing.T) {
	r, _ := wiredRouter(t, true, nil)
	w := hit(r, http.MethodGet, "/api/questions", "")
	if w.Code != http.StatusOK {
		t.Fatalf("HTTP 状态应保持 200（错误码在 body 里），实际 %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), `"code":503`) {
		t.Fatalf("维护期间业务接口应返回业务码 503，实际 %s", w.Body.String())
	}
}

// c.Abort() 必须真的中止后续 handler，且被挡的响应里只有维护错误本身。
//
// 这条断言有两重牙齿（都实测过）：
//  1. 删掉中间件里的 c.Abort()，请求会继续往下走 —— 本 router 里 /api/questions
//     挂在 mw.Auth 之后，于是响应体变成「维护错误 + 认证错误」两段拼接，
//     用例失败。维护期间写库的 handler 绝不执行，是 Abort 唯一的职责。
//  2. body 被钉成字面量：判别位 data.maintenance 或文案被删改，同样失败。
func TestMaintenanceBlockAbortsDownstreamHandler(t *testing.T) {
	r, _ := wiredRouter(t, true, nil)
	w := hit(r, http.MethodGet, "/api/questions", "")

	const want = `{"code":503,"message":"系统维护中","data":{"maintenance":true}}`
	if got := w.Body.String(); got != want {
		t.Fatalf("被挡的响应应当只有维护错误本身（下游 handler 不得运行）\n期望 %s\n实际 %s", want, got)
	}
}

// 预检请求不带 Authorization。维护中间件若排在 Cors 之前，预检会被它挡下，
// 浏览器只会报跨域失败 —— 用户看不到维护提示，前端错误处理也没机会跑。
// 这个断言能区分顺序：顺序反了 body 会是 503、状态码不再 204、且没有 CORS 头。
func TestCorsPreflightSurvivesMaintenance(t *testing.T) {
	r, _ := wiredRouter(t, true, nil)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodOptions, "/api/questions", nil)
	req.Header.Set("Origin", "https://example.com")
	r.ServeHTTP(w, req)

	if strings.Contains(w.Body.String(), `"code":503`) {
		t.Fatalf("预检被维护中间件拦下，浏览器只会报跨域失败而非维护提示：%s", w.Body.String())
	}
	if w.Code != http.StatusNoContent {
		t.Fatalf("预检应由 Cors 以 204 结束，实际 %d %s", w.Code, w.Body.String())
	}
	if w.Header().Get("Access-Control-Allow-Methods") == "" {
		t.Fatalf("预检响应缺少 CORS 头：%v", w.Header())
	}
}

// 后台路由要挂在 mw.Auth 之后；且维护期间 /api/admin 在白名单里，
// 所以这里必须是 401 而不是 503 —— 否则开关一开管理员再也关不掉它。
func TestAdminSiteSettingsNeedsAuth(t *testing.T) {
	for _, maintenance := range []bool{false, true} {
		r, _ := wiredRouter(t, maintenance, nil)
		w := hit(r, http.MethodGet, "/api/admin/site-settings", "")
		if !strings.Contains(w.Body.String(), `"code":401`) {
			t.Fatalf("maintenance=%v 时未带 token 应返回业务码 401，实际 %d %s",
				maintenance, w.Code, w.Body.String())
		}
	}
}

// adminReq 带管理员 token 发请求，body 原样传入（传空串表示无 body）。
func adminReq(r *gin.Engine, method, path, token, body string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	var rd io.Reader
	if body != "" {
		rd = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, path, rd)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	return w
}

// 端到端：管理员拨开关 → 前端读得到、业务接口被挡 → 拨回去 → 恢复。
// 走真路由 + 真 Auth/Admin 中间件，唯一替身是仓储。
func TestAdminToggleFlipsStatusAndGate(t *testing.T) {
	admin := &model.User{ID: 21, Username: "admin21", Role: 1, Status: 1}
	r, _ := wiredRouter(t, false, admin)
	tok := adminToken(t, admin)

	put := func(enabled bool) string {
		return adminReq(r, http.MethodPut, "/api/admin/site-settings", tok,
			fmt.Sprintf(`{"maintenance_enabled":%v}`, enabled)).Body.String()
	}
	// 后台卡片初始状态就读这个接口，两个状态下都得答对
	settings := func() string {
		return adminReq(r, http.MethodGet, "/api/admin/site-settings", tok, "").Body.String()
	}

	if got := put(true); !strings.Contains(got, `"maintenance_enabled":true`) {
		t.Fatalf("开启维护失败：%s", got)
	}
	if got := settings(); !strings.Contains(got, `"maintenance_enabled":true`) {
		t.Fatalf("开关已开，后台读到的仍是旧值：%s", got)
	}
	if got := hit(r, http.MethodGet, "/api/site-status", "").Body.String(); !strings.Contains(got, `"maintenance":true`) {
		t.Fatalf("开关已开，前端读到的仍是旧值：%s", got)
	}
	if got := hit(r, http.MethodGet, "/api/questions", "").Body.String(); !strings.Contains(got, `"code":503`) {
		t.Fatalf("开关已开，业务接口未被挡下：%s", got)
	}

	if got := put(false); !strings.Contains(got, `"maintenance_enabled":false`) {
		t.Fatalf("关闭维护失败：%s", got)
	}
	if got := settings(); !strings.Contains(got, `"maintenance_enabled":false`) {
		t.Fatalf("开关已关，后台仍读到维护中：%s", got)
	}
	if got := hit(r, http.MethodGet, "/api/site-status", "").Body.String(); !strings.Contains(got, `"maintenance":false`) {
		t.Fatalf("开关已关，前端仍读到维护中：%s", got)
	}
}

// 缺 maintenance_enabled 字段的请求体（最典型的是 {}）必须被拒，而不是
// 被绑成 false 静默把维护关掉 —— 一个写错的请求体不该有开启/关闭闸门的效果。
//
// 只断言 400 是不够的：还得回头确认开关没动，否则「先 SetMaintenance(false)
// 再返回 400」这种实现照样能过。所以这里同时看服务视角（GET settings）和
// 落库视角（仓储的 on），并在**维护开着**的状态下跑 —— 那样意外绑成 false
// 才会表现为一次可见的翻转，而不是「本来就没开，看不出来」。
func TestAdminSiteSettingsRejectsMissingFieldWithoutFlipping(t *testing.T) {
	admin := &model.User{ID: 21, Username: "admin21", Role: 1, Status: 1}
	r, repo := wiredRouter(t, true, admin)
	tok := adminToken(t, admin)

	w := adminReq(r, http.MethodPut, "/api/admin/site-settings", tok, `{}`)
	if !strings.Contains(w.Body.String(), `"code":400`) {
		t.Fatalf("缺字段的请求体应返回业务码 400，实际 %s", w.Body.String())
	}

	if got := adminReq(r, http.MethodGet, "/api/admin/site-settings", tok, "").Body.String(); !strings.Contains(got, `"maintenance_enabled":true`) {
		t.Fatalf("被拒的请求改动了开关（服务视角）：%s", got)
	}
	if !repo.on {
		t.Fatalf("被拒的请求改动了开关（仓储视角）：维护在落库层面被关掉了")
	}
}
