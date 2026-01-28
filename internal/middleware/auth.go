package middleware

import (
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/kyangconn/music-online-web/internal/handler"
	"github.com/kyangconn/music-online-web/internal/pkg/jwt"
)

func AuthMiddleware() gin.HandlerFunc {
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
			if err == jwt.ErrExpiredToken {
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
