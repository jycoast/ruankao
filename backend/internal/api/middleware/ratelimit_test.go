package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"software-exam/config"

	"github.com/gin-gonic/gin"
)

// Redis 未初始化时限流应 fail-open，全部放行
func TestRateLimitFailOpenWithoutRedis(t *testing.T) {
	gin.SetMode(gin.TestMode)
	config.AppConfig.AntiCrawl.Enable = true

	r := gin.New()
	r.GET("/t", func(c *gin.Context) {
		c.Set("user_id", uint64(1))
	}, RateLimitGlobal(), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	for i := 0; i < 200; i++ {
		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/t", nil)
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("request %d: expected 200 (fail-open), got %d", i, w.Code)
		}
	}
}

// 总开关关闭时不应做任何限流
func TestRateLimitDisabled(t *testing.T) {
	gin.SetMode(gin.TestMode)
	config.AppConfig.AntiCrawl.Enable = false
	defer func() { config.AppConfig.AntiCrawl.Enable = true }()

	r := gin.New()
	r.GET("/t", RateLimitByIP(), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/t", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 when disabled, got %d", w.Code)
	}
}

// 配置缺省时应回填默认值
func TestAcConfigDefaults(t *testing.T) {
	config.AppConfig.AntiCrawl = config.AntiCrawlConfig{Enable: true}
	cfg := acConfig()
	if cfg.GlobalPerMin != 120 || cfg.DataPerMin != 30 || cfg.AuthPerMin != 10 ||
		cfg.SuspectPerMin != 10 || cfg.DistinctQuestions24h != 500 || cfg.SequentialThreshold != 45 {
		t.Fatalf("unexpected defaults: %+v", cfg)
	}
}

// 虚拟游客(user_id=0)的计费键必须落在匿名公开桶 rl:ip:{ip}:public,不能按 rl:u:0
// (全站游客共用一个用户桶会互相饿死),也不能直接放行(游客带 Auth 而匿名公开走
// RateLimitPublic,只有游客会以 uid=0 到达 RateLimitByUser)。
func TestGuestRateKey_FallsBackToIPPublic(t *testing.T) {
	if k := guestPublicKey("1.2.3.4"); k != "rl:ip:1.2.3.4:public" {
		t.Fatalf("游客键 = %q, want rl:ip:1.2.3.4:public", k)
	}
}

// AI 专用限额与并发上限的默认值
func TestAcConfigAIDefaults(t *testing.T) {
	old := config.AppConfig.AntiCrawl
	defer func() { config.AppConfig.AntiCrawl = old }()
	config.AppConfig.AntiCrawl = config.AntiCrawlConfig{Enable: true}

	cfg := acConfig()
	if cfg.AIPerMin != 10 || cfg.AIMaxConcurrent != 2 {
		t.Fatalf("AI 默认值不符: perMin=%d maxConcurrent=%d", cfg.AIPerMin, cfg.AIMaxConcurrent)
	}
}

// Redis 未初始化时 AI 闸门必须 fail-open
func TestAIRateLimitFailOpenWithoutRedis(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/t", func(c *gin.Context) { c.Set("user_id", uint64(1)) },
		RateLimitAI(), AIConcurrency(), func(c *gin.Context) { c.Status(http.StatusOK) })

	for i := 0; i < 50; i++ {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest("GET", "/t", nil))
		if w.Code != http.StatusOK {
			t.Fatalf("request %d: expected 200 (fail-open), got %d", i, w.Code)
		}
	}
}
