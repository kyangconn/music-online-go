package middleware

import (
	"errors"

	"github.com/gin-gonic/gin"
	"github.com/kyangconn/music-online-web/internal/domain"
	"github.com/kyangconn/music-online-web/internal/handler"
	"gorm.io/gorm"
)

// StrictAdminMiddleware checks admin status against database for every request
// This prevents token reuse if the user's role is revoked or account is banned
func StrictAdminMiddleware(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 1. Get UserID from context (set by AuthMiddleware)
		userID, exists := c.Get("userID")
		if !exists {
			handler.Unauthorized(c, "Unauthorized")
			c.Abort()
			return
		}

		// 2. Query Database
		var user domain.User
		if err := db.Select("id", "role", "is_active").First(&user, userID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				handler.Unauthorized(c, "User not found")
			} else {
				handler.InternalServerError(c, "Database error")
			}
			c.Abort()
			return
		}

		// 3. Check Role and Status
		if !user.IsActive {
			handler.Forbidden(c, "Account is inactive")
			c.Abort()
			return
		}

		if user.Role != "admin" {
			handler.Forbidden(c, "Requires admin privileges")
			c.Abort()
			return
		}

		c.Next()
	}
}
