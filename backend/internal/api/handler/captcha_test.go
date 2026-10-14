package handler

import (
	"encoding/json"
	"errors"
	"net/http/httptest"
	"strings"
	"testing"

	"software-exam/pkg/redisclient"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

// Redis 不可用时 Verify 必须 fail-open，否则 Redis 故障会把注册全部锁死。
func TestVerifyCaptchaFailOpenWhenRedisDown(t *testing.T) {
	old := redisclient.Client
	redisclient.Client = nil
	defer func() { redisclient.Client = old }()

	if !VerifyCaptcha("any-id", "any-code") {
		t.Fatal("expected fail-open (true) when redis client is nil")
	}
}

func TestCaptchaAnswerEqual(t *testing.T) {
	if !captchaAnswerEqual("AbC4", "abc4") {
		t.Fatal("expected case-insensitive match")
	}
	if captchaAnswerEqual("", "abc4") {
		t.Fatal("empty stored answer must not match")
	}
	if captchaAnswerEqual("AbC4", "xyz") {
		t.Fatal("wrong answer must not match")
	}
}

// verifyWithGetResult 纯决策表：redis.Nil=真「无验证码」→ false；
// 其余错误（Redis 故障）→ fail-open true；命中则大小写不敏感比较。
func TestVerifyWithGetResult(t *testing.T) {
	cases := []struct {
		name   string
		val    string
		err    error
		answer string
		want   bool
	}{
		{"key not found (captcha absent/consumed/expired)", "", redis.Nil, "1234", false},
		{"redis down fails open", "", errors.New("redis down"), "1234", true},
		{"case-insensitive match", "AbC4", nil, "abc4", true},
		{"empty stored never matches", "", nil, "1234", false},
		{"wrong answer", "AbC4", nil, "xyz", false},
	}
	for _, tc := range cases {
		if got := verifyWithGetResult(tc.val, tc.err, tc.answer); got != tc.want {
			t.Errorf("%s: verifyWithGetResult(%q, %v, %q)=%v, want %v",
				tc.name, tc.val, tc.err, tc.answer, got, tc.want)
		}
	}
}

// 响应形状回归测试：image_base64 必须是完整 data URI，且前缀只出现一次
// （防止回归到「前端拼前缀」导致双 data: 前缀破图）。
func TestGetCaptchaResponseShape(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/auth/captcha", nil)

	GetCaptcha(c)

	var body struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Data    struct {
			CaptchaID   string `json:"captcha_id"`
			ImageBase64 string `json:"image_base64"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if body.Code != 0 {
		t.Fatalf("expected code 0, got %d (%s)", body.Code, body.Message)
	}
	if body.Data.CaptchaID == "" {
		t.Fatal("expected non-empty captcha_id")
	}

	const prefix = "data:image/png;base64,"
	if !strings.HasPrefix(body.Data.ImageBase64, prefix) {
		got := body.Data.ImageBase64
		if len(got) > 60 {
			got = got[:60]
		}
		t.Fatalf("image_base64 must start with %q, got %q", prefix, got)
	}
	if strings.Contains(body.Data.ImageBase64, prefix+prefix) {
		t.Fatal("image_base64 must not contain the data prefix twice")
	}
}
