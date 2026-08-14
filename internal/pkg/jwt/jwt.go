// Package jwt jwt.go - JWT 令牌管理
// 提供短期 access token 的生成和解析功能。access token 只携带身份与
// 会话引用，会话生命周期（轮换、过期、撤销）由服务端 sessions 表管理。
package jwt

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v4"
)

var (
	ErrInvalidToken = errors.New("invalid token")
	ErrExpiredToken = errors.New("token has expired")
)

const tokenIssuer = "user-management"

type Claims struct {
	UserID    uint   `json:"user_id"`
	Username  string `json:"username"`
	Role      string `json:"role"`
	SessionID uint   `json:"session_id"`
	jwt.RegisteredClaims
}

// GenerateToken 生成短期 access token。secret 与 TTL 由调用方从已验证的
// 启动配置快照传入，避免服务层隐式依赖全局配置。
func GenerateToken(userID uint, username, role string, sessionID uint, ttlMinutes int, secret string) (string, error) {
	expireTime := time.Now().Add(time.Duration(ttlMinutes) * time.Minute)

	claims := Claims{
		UserID:    userID,
		Username:  username,
		Role:      role,
		SessionID: sessionID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expireTime),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    tokenIssuer,
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}

// ParseToken 解析和验证JWT令牌
func ParseToken(tokenString string, secret string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if token.Method != jwt.SigningMethodHS256 {
			return nil, ErrInvalidToken
		}
		return []byte(secret), nil
	}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}))

	if err != nil {
		if ve, ok := errors.AsType[*jwt.ValidationError](err); ok {
			if ve.Errors&jwt.ValidationErrorExpired != 0 {
				return nil, ErrExpiredToken
			}
		}
		return nil, ErrInvalidToken
	}

	if claims, ok := token.Claims.(*Claims); ok && token.Valid && claims.VerifyIssuer(tokenIssuer, true) {
		return claims, nil
	}

	return nil, ErrInvalidToken
}
