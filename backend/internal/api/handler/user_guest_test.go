package handler

import (
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

// 游客(user_id=0,role=2)直调个人资料:在调用 service 前短路回 401,不 500。
// userService 置 nil——游客分支若误调用 service 会 panic,从而证明短路确实发生。
func TestUserHandler_GetProfile_GuestNot500(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := &UserHandler{} // userService = nil
	r := gin.New()
	r.GET("/user/profile", func(c *gin.Context) {
		c.Set("user_id", uint64(0))
		c.Set("role", int8(2))
		h.GetProfile(c)
	})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/user/profile", nil))

	var body struct {
		Code int `json:"code"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("body 非 JSON: %v", err)
	}
	if body.Code != 401 {
		t.Fatalf("游客 GetProfile 应 401,got %d (%s)", body.Code, w.Body.String())
	}
}
