package service

import (
	"testing"

	"software-exam/pkg/auth"
)

// GuestLogin 不碰任何 repo,直接签游客令牌。
func TestUserService_GuestLogin(t *testing.T) {
	s := &UserService{} // 零值即可:方法不使用 repo
	tok, err := s.GuestLogin()
	if err != nil {
		t.Fatalf("GuestLogin: %v", err)
	}
	claims, err := auth.ParseToken(tok)
	if err != nil {
		t.Fatalf("ParseToken: %v", err)
	}
	if claims.Role != auth.RoleGuest || claims.UserID != 0 {
		t.Fatalf("游客 claim 错: %+v", claims)
	}
}
