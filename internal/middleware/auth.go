// Package middleware auth.go - 认证中间件
// JWT 令牌验证与会话有效性检查，将用户信息注入请求上下文
package middleware

import (
	"errors"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/kyangconn/music-online-go/internal/domain"
	"github.com/kyangconn/music-online-go/internal/handler"
	"github.com/kyangconn/music-online-go/internal/pkg/jwt"
)

func AuthMiddleware(db *gorm.DB, jwtSecret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			handler.Unauthorized(c, "Authorization header is required")
			c.Abort()
			return
		}

		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || parts[0] != "Bearer" {
			handler.Unauthorized(c, "Authorization header format must be Bearer {token}")
			c.Abort()
			return
		}

		claims, err := jwt.ParseToken(parts[1], jwtSecret)
		if err != nil {
			if errors.Is(err, jwt.ErrExpiredToken) {
				handler.Unauthorized(c, "Token has expired")
			} else {
				handler.Unauthorized(c, "Invalid token")
			}
			c.Abort()
			return
		}

		// P0: verify account is still active so disabled users' existing JWTs are invalidated
		var user domain.User
		if err := db.Select("is_active").First(&user, claims.UserID).Error; err != nil || !user.IsActive {
			handler.Unauthorized(c, "Account is disabled or not found")
			c.Abort()
			return
		}

		// P0: verify the server-side session still exists and is not revoked so
		// single-device/all-devices logout takes effect immediately.
		if !sessionIsActive(db, claims.SessionID) {
			handler.Unauthorized(c, "Session has been revoked")
			c.Abort()
			return
		}

		// 将用户信息存储到上下文中
		c.Set("userID", claims.UserID)
		c.Set("username", claims.Username)
		c.Set("role", claims.Role)
		c.Set("sessionID", claims.SessionID)

		c.Next()
	}
}

// OptionalAuthMiddleware 尝试解析Token，如果失败也不阻止请求。已撤销或
// 缺失的会话会被忽略，避免登出后残留的本地令牌继续暴露登录态。
func OptionalAuthMiddleware(db *gorm.DB, jwtSecret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader != "" {
			parts := strings.Split(authHeader, " ")
			if len(parts) == 2 && parts[0] == "Bearer" {
				claims, err := jwt.ParseToken(parts[1], jwtSecret)
				if err == nil && sessionIsActive(db, claims.SessionID) {
					c.Set("userID", claims.UserID)
					c.Set("username", claims.Username)
					c.Set("role", claims.Role)
					c.Set("sessionID", claims.SessionID)
				}
			}
		}
		c.Next()
	}
}

// sessionIsActive reports whether the session exists and has not been revoked.
// Tokens minted before sessions existed (sessionID == 0) are never accepted.
func sessionIsActive(db *gorm.DB, sessionID uint) bool {
	if sessionID == 0 {
		return false
	}
	var session domain.Session
	if err := db.Select("revoked_at").First(&session, sessionID).Error; err != nil {
		return false
	}
	return session.RevokedAt == nil
}

// RoleMiddleware 角色权限中间件
func RoleMiddleware(allowedRoles ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		userRole, exists := c.Get("role")
		if !exists {
			handler.Unauthorized(c, "Unauthorized")
			c.Abort()
			return
		}

		// 检查用户角色是否在允许的角色列表中
		for _, role := range allowedRoles {
			if role == userRole {
				c.Next()
				return
			}
		}

		handler.Forbidden(c, "Insufficient permissions")
		c.Abort()
	}
}
