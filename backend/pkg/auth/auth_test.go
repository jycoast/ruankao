package auth

import (
	"testing"
	"time"

	"software-exam/config"
)

// GenerateGuestToken / ParseToken 都读全局 config.AppConfig(config.go:152 的 struct 值),
// 直接改全局并在测试结束后还原(不能拷 struct——改副本不生效)。
func TestGenerateGuestToken_ClaimsAndShortTTL(t *testing.T) {
	orig := config.AppConfig
	t.Cleanup(func() { config.AppConfig = orig })
	config.AppConfig.JWT.Secret = "unit-test-secret-for-guest-token-0123456789"
	config.AppConfig.JWT.ExpireHour = 72     // 普通用户档(应被游客 TTL 覆盖)
	config.AppConfig.JWT.GuestExpireHour = 2 // 游客短 TTL

	tok, err := GenerateGuestToken()
	if err != nil {
		t.Fatalf("GenerateGuestToken: %v", err)
	}
	claims, err := ParseToken(tok)
	if err != nil {
		t.Fatalf("ParseToken: %v", err)
	}
	if claims.Role != RoleGuest {
		t.Fatalf("role = %d, want %d", claims.Role, RoleGuest)
	}
	if claims.UserID != 0 {
		t.Fatalf("user_id = %d, want 0", claims.UserID)
	}
	if claims.Username != GuestUsername {
		t.Fatalf("username = %q, want %q", claims.Username, GuestUsername)
	}
	// 游客 TTL 走 guest_expire_hour(2h);若误用普通用户 72h,time.Until 会远超 3h
	if claims.ExpiresAt == nil || time.Until(claims.ExpiresAt.Time) > 3*time.Hour {
		t.Fatalf("游客令牌过期时间异常: %v", claims.ExpiresAt)
	}
}
