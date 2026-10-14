package auth

import (
	"time"

	"software-exam/config"

	"github.com/golang-jwt/jwt/v5"
)

type Claims struct {
	UserID   uint64 `json:"user_id"`
	Username string `json:"username"`
	Role     int8   `json:"role"`
	jwt.RegisteredClaims
}

// 虚拟游客角色:仅存在于令牌与请求内存,DB 里没有 role=2 的用户。
// AuthMiddleware 据此跳过用户状态回读并强制只读。
const (
	RoleGuest         int8   = 2
	GuestUsername     string = "guest"
	DefaultGuestHours        = 24
)

func guestExpireHours() int {
	h := config.AppConfig.JWT.GuestExpireHour
	if h <= 0 {
		h = DefaultGuestHours
	}
	return h
}

// GenerateGuestToken 签发虚拟游客令牌(user_id=0, role=2, 短 TTL)。不落任何表。
func GenerateGuestToken() (string, error) {
	claims := Claims{
		UserID:   0,
		Username: GuestUsername,
		Role:     RoleGuest,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Duration(guestExpireHours()) * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(config.AppConfig.JWT.Secret))
}

func GenerateToken(userID uint64, username string, role int8) (string, error) {
	expireHour := config.AppConfig.JWT.ExpireHour
	claims := Claims{
		UserID:   userID,
		Username: username,
		Role:     role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Duration(expireHour) * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(config.AppConfig.JWT.Secret))
}

func ParseToken(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		return []byte(config.AppConfig.JWT.Secret), nil
	})
	if err != nil {
		return nil, err
	}
	if claims, ok := token.Claims.(*Claims); ok && token.Valid {
		return claims, nil
	}
	return nil, jwt.ErrSignatureInvalid
}
