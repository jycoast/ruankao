package handler

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"software-exam/pkg/auth"

	"github.com/gin-gonic/gin"
)

// silentRecovery 把 handler 内 panic 静默兜成 500(不打印栈),便于 nil-service 正向对照测试。
func silentRecovery() gin.HandlerFunc {
	return gin.CustomRecoveryWithWriter(io.Discard, func(c *gin.Context, _ interface{}) {
		c.AbortWithStatus(http.StatusInternalServerError)
	})
}

// 游客(role=2)请求 AI 状态:须在任何 service 调用前短路为不可用态
// (enabled=false、无试用配额、own_config=false)。aiService/membershipService 置 nil——
// 游客分支若误调 service 会 panic(recovery 兜成 500),从而证明短路确实发生在服务调用之前。
func TestAIHandler_Status_GuestUnavailable(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := &AIHandler{} // aiService / membershipService = nil
	r := gin.New()
	r.Use(silentRecovery())
	r.GET("/ai/status", func(c *gin.Context) {
		c.Set("user_id", uint64(0))
		c.Set("role", int8(auth.RoleGuest))
		h.Status(c)
	})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/ai/status", nil))

	if w.Code != http.StatusOK {
		t.Fatalf("HTTP = %d, want 200 (%s)", w.Code, w.Body.String())
	}
	var body struct {
		Code int `json:"code"`
		Data struct {
			Enabled    bool `json:"enabled"`
			TrialUsed  int  `json:"trial_used"`
			TrialLimit int  `json:"trial_limit"`
			OwnConfig  bool `json:"own_config"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("body 非 JSON: %v (%s)", err, w.Body.String())
	}
	if body.Code != 0 {
		t.Fatalf("code = %d, want 0 (%s)", body.Code, w.Body.String())
	}
	if body.Data.Enabled {
		t.Fatalf("游客 AI enabled 应为 false (%s)", w.Body.String())
	}
	if body.Data.TrialLimit != 0 || body.Data.TrialUsed != 0 {
		t.Fatalf("游客不应展示试用配额: %+v", body.Data)
	}
	if body.Data.OwnConfig {
		t.Fatalf("游客 own_config 应为 false")
	}
}

// 正向对照:role=1(管理员,非游客)不得被游客短路吞掉。nil aiService 下非游客路径会深入
// 服务层而 panic(recovery 兜成 500,响应非游客不可用体),证明该请求没命中游客分支——
// 防止守卫被误放宽到全体角色(若删掉 `== auth.RoleGuest`,role=1 会返回游客体 → 本测试转红)。
func TestAIHandler_Status_NonGuestDoesNotTakeGuestBranch(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := &AIHandler{} // aiService = nil
	r := gin.New()
	r.Use(silentRecovery())
	r.GET("/ai/status", func(c *gin.Context) {
		c.Set("user_id", uint64(1))
		c.Set("role", int8(1))
		h.Status(c)
	})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/ai/status", nil))

	if w.Code == http.StatusOK && guestUnavailableBody(w.Body.String()) {
		t.Fatalf("role=1 被游客短路吞掉(游客不可用体只应发给 role 2): %s", w.Body.String())
	}
}

// guestUnavailableBody 判断 body 是否为游客不可用成功体(200 + enabled:false + 零配额 + own_config:false)。
func guestUnavailableBody(body string) bool {
	var parsed struct {
		Code int `json:"code"`
		Data struct {
			Enabled    bool `json:"enabled"`
			TrialUsed  int  `json:"trial_used"`
			TrialLimit int  `json:"trial_limit"`
			OwnConfig  bool `json:"own_config"`
		} `json:"data"`
	}
	if json.Unmarshal([]byte(body), &parsed) != nil {
		return false
	}
	return parsed.Code == 0 && !parsed.Data.Enabled &&
		parsed.Data.TrialUsed == 0 && parsed.Data.TrialLimit == 0 && !parsed.Data.OwnConfig
}
