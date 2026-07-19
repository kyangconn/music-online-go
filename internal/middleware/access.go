package middleware

import (
	"crypto/subtle"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/kyangconn/music-online-go/internal/config"
	"github.com/kyangconn/music-online-go/internal/domain"
	"github.com/kyangconn/music-online-go/internal/handler"
	"github.com/kyangconn/music-online-go/internal/pkg/mediatoken"
)

func LibraryReadMiddleware(db *gorm.DB, mode string) gin.HandlerFunc {
	if mode == config.LibraryAccessAuthenticated {
		authenticate := AuthMiddleware(db)
		return func(c *gin.Context) {
			c.Header("Cache-Control", "private, no-store")
			authenticate(c)
		}
	}
	return OptionalAuthMiddleware()
}

func LibraryMediaAccessMiddleware(db *gorm.DB, mode, secret, kind string) gin.HandlerFunc {
	if mode != config.LibraryAccessAuthenticated {
		return OptionalAuthMiddleware()
	}
	return func(c *gin.Context) {
		c.Header("Cache-Control", "private, no-store")
		musicID, err := strconv.ParseUint(c.Param("id"), 10, 32)
		if err == nil {
			token := c.Query("media_token")
			if token != "" && mediatoken.Validate(token, secret, uint(musicID), kind, time.Now()) == nil {
				c.Next()
				return
			}
		}
		AuthMiddleware(db)(c)
	}
}

func MusicBeeSubmitTokenMiddleware(db *gorm.DB, cfg config.MusicBeeConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		if strings.TrimSpace(cfg.SubmitToken) == "" || strings.TrimSpace(cfg.SubmitUsername) == "" {
			c.AbortWithStatus(http.StatusNotFound)
			return
		}
		provided, ok := bearerToken(c.GetHeader("Authorization"))
		if !ok || subtle.ConstantTimeCompare([]byte(provided), []byte(cfg.SubmitToken)) != 1 {
			c.Header("WWW-Authenticate", `Bearer scope="track:submit"`)
			handler.Unauthorized(c, "Invalid MusicBee submit credential")
			c.Abort()
			return
		}

		var user domain.User
		if err := db.Select("id", "username", "role", "is_active").
			Where("username = ?", cfg.SubmitUsername).
			First(&user).Error; err != nil || !user.IsActive {
			handler.Unauthorized(c, "MusicBee submit user is inactive or unavailable")
			c.Abort()
			return
		}
		c.Set("userID", user.ID)
		c.Set("username", user.Username)
		c.Set("role", user.Role)
		c.Set("credentialScope", "track:submit")
		c.Next()
	}
}

func bearerToken(header string) (string, bool) {
	const prefix = "Bearer "
	if !strings.HasPrefix(header, prefix) {
		return "", false
	}
	token := strings.TrimSpace(strings.TrimPrefix(header, prefix))
	return token, token != ""
}
