package middleware

import (
	"encoding/json"
	"net/http/httptest"
	"testing"

	"software-exam/config"
	"software-exam/internal/application/service"
	"software-exam/pkg/auth"

	"github.com/gin-gonic/gin"
)

// 游客分支在 ParseToken 后立即短路,不会触碰 userState(传零值即可证明不被调用)。
func TestAuthMiddleware_GuestReadOnly(t *testing.T) {
	gin.SetMode(gin.TestMode)

	orig := config.AppConfig
	t.Cleanup(func() { config.AppConfig = orig })
	config.AppConfig.JWT.Secret = "guest-middleware-test-secret-at-least-32-chars!!"
	guestTok, err := auth.GenerateGuestToken()
	if err != nil {
		t.Fatalf("GenerateGuestToken: %v", err)
	}

	// 每次请求都返回这个中间件;userState 是零值(内部 nil repo),若游客分支误回读会 panic
	mw := AuthMiddleware(&service.UserStateService{})

	var gotRole int8
	r := gin.New()
	r.GET("/echo", mw, func(c *gin.Context) {
		if v, ok := c.Get("role"); ok {
			gotRole = v.(int8)
		}
		if c.GetUint64("user_id") != 0 {
			t.Fatalf("游客 user_id 应为 0")
		}
		c.String(200, "ok")
	})

	// GET(读)→ 放行
	req := httptest.NewRequest("GET", "/echo", nil)
	req.Header.Set("Authorization", "Bearer "+guestTok)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("游客 GET 应放行,got %d %s", w.Code, w.Body.String())
	}
	if gotRole != auth.RoleGuest {
		t.Fatalf("context role = %d, want %d", gotRole, auth.RoleGuest)
	}

	// POST(写)→ 403
	r.POST("/write", mw, func(c *gin.Context) { c.String(200, "不该到这") })
	req2 := httptest.NewRequest("POST", "/write", nil)
	req2.Header.Set("Authorization", "Bearer "+guestTok)
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)
	var body struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(w2.Body.Bytes(), &body); err != nil {
		t.Fatalf("body 非 JSON: %v", err)
	}
	if body.Code != 403 {
		t.Fatalf("游客 POST 应 403,got %+v", body)
	}
}
