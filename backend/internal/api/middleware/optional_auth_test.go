package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"software-exam/internal/application/service"
	"software-exam/internal/domain/model"
	"software-exam/pkg/auth"

	"github.com/gin-gonic/gin"
)

// optionalRouter 回显中间件解析出的身份，0 表示按匿名放行。
func optionalRouter(repo *fakeUserRepo) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/t", OptionalAuth(service.NewUserStateService(repo)), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"uid": c.GetUint64("user_id")})
	})
	return r
}

func getWithToken(r *gin.Engine, token string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/t", nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	r.ServeHTTP(w, req)
	return w
}

// 无 token 必须放行 —— 公开内容页要能被爬虫和游客读到。
func TestOptionalAuthAllowsAnonymous(t *testing.T) {
	w := getWithToken(optionalRouter(&fakeUserRepo{}), "")
	if w.Code != http.StatusOK || w.Body.String() != `{"uid":0}` {
		t.Fatalf("anonymous should pass as uid=0, got %d %s", w.Code, w.Body.String())
	}
}

// 有效 token 仍要把身份写进 context，否则公开页上的登录态功能（收藏等）会失效。
func TestOptionalAuthSetsIdentity(t *testing.T) {
	u := &model.User{ID: 11, Username: "u11", Role: 0, Status: 1}
	tok := tokenFor(t, u)
	w := getWithToken(optionalRouter(&fakeUserRepo{user: u}), tok)
	if w.Body.String() != `{"uid":11}` {
		t.Fatalf("valid token should set user_id, got %s", w.Body.String())
	}
}

// 与 AuthMiddleware 相反：封号/无效 token 降级为匿名而非拒绝。
// 公开内容本就人人可读，这里 fail-closed 只会让公开页跟着数据库一起挂。
func TestOptionalAuthDegradesBannedAndInvalidToAnonymous(t *testing.T) {
	u := &model.User{ID: 12, Username: "u12", Role: 0, Status: 0} // 已封号
	tok := tokenFor(t, u)

	for name, tc := range map[string]struct {
		repo  *fakeUserRepo
		token string
	}{
		"banned":       {&fakeUserRepo{user: u}, tok},
		"deleted":      {&fakeUserRepo{}, tok},
		"garbageToken": {&fakeUserRepo{user: u}, "not-a-jwt"},
	} {
		w := getWithToken(optionalRouter(tc.repo), tc.token)
		if w.Code != http.StatusOK || w.Body.String() != `{"uid":0}` {
			t.Fatalf("%s should degrade to anonymous, got %d %s", name, w.Code, w.Body.String())
		}
	}
}

// 游客 token(role 2)必须降级为匿名。
// 这条分支是 resolveIdentity 重构时新加的显式判断，此前是「查库查不到 → 匿名」
// 顺带得到的，谁也不知道它是不是刻意的。现在它是刻意的，且有测试钉住。
func TestOptionalAuthDegradesGuestToAnonymous(t *testing.T) {
	tok := tokenFor(t, &model.User{ID: 0, Username: auth.GuestUsername, Role: auth.RoleGuest})
	w := getWithToken(optionalRouter(&fakeUserRepo{}), tok)
	if w.Code != http.StatusOK || w.Body.String() != `{"uid":0}` {
		t.Fatalf("guest should degrade to anonymous, got %d %s", w.Code, w.Body.String())
	}
}
