// Package middleware auth.go - 认证中间件
// JWT 令牌验证，将用户信息注入请求上下文
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

func AuthMiddleware(db *gorm.DB) gin.HandlerFunc {
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

		claims, err := jwt.ParseToken(parts[1])
		if err != nil {
			if errors.Is(err, jwt.ErrExpiredToken) {
				handler.Unauthorized(c, "Token has expired")
			} else {
				handler.Unauthorized(c, "Invalid token")
			}
			c.Abort()
			return
		}

		// 将用户信息存储到上下文中
		c.Set("userID", claims.UserID)
		c.Set("username", claims.Username)
		c.Set("role", claims.Role)

		// P0: verify account is still active so disabled users' existing JWTs are invalidated
		var user domain.User
		if err := db.Select("is_active").First(&user, claims.UserID).Error; err != nil || !user.IsActive {
			handler.Unauthorized(c, "Account is disabled or not found")
			c.Abort()
			return
		}

		c.Next()
	}
}

// OptionalAuthMiddleware 尝试解析Token，如果失败也不阻止请求
func OptionalAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader != "" {
			parts := strings.Split(authHeader, " ")
			if len(parts) == 2 && parts[0] == "Bearer" {
				claims, err := jwt.ParseToken(parts[1])
				if err == nil {
					c.Set("userID", claims.UserID)
					c.Set("username", claims.Username)
					c.Set("role", claims.Role)
				}
			}
		}
		c.Next()
	}
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
