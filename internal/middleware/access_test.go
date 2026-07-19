package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"github.com/kyangconn/music-online-go/internal/config"
	"github.com/kyangconn/music-online-go/internal/domain"
	"github.com/kyangconn/music-online-go/internal/pkg/jwt"
	"github.com/kyangconn/music-online-go/internal/pkg/mediatoken"
)

func accessTestDatabase(t *testing.T) (*gorm.DB, domain.User, string) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("open SQL database: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)
	if err := db.AutoMigrate(&domain.User{}); err != nil {
		t.Fatalf("migrate user: %v", err)
	}
	user := domain.User{Username: "listener", Email: "listener@example.com", Password: "unused", Role: "user", IsActive: true}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
	token, err := jwt.GenerateToken(user.ID, user.Username, user.Role)
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}
	return db, user, token
}

func TestLibraryReadMiddlewareModes(t *testing.T) {
	db, _, token := accessTestDatabase(t)
	serve := func(mode, authorization string) int {
		router := gin.New()
		router.GET("/library", LibraryReadMiddleware(db, mode), func(c *gin.Context) { c.Status(http.StatusOK) })
		request := httptest.NewRequest(http.MethodGet, "/library", nil)
		if authorization != "" {
			request.Header.Set("Authorization", authorization)
		}
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)
		return response.Code
	}

	if got := serve(config.LibraryAccessPublic, ""); got != http.StatusOK {
		t.Fatalf("public anonymous status = %d", got)
	}
	if got := serve(config.LibraryAccessAuthenticated, ""); got != http.StatusUnauthorized {
		t.Fatalf("authenticated anonymous status = %d", got)
	}
	if got := serve(config.LibraryAccessAuthenticated, "Bearer "+token); got != http.StatusOK {
		t.Fatalf("authenticated bearer status = %d", got)
	}
}

func TestLibraryMediaAccessAcceptsOnlyBoundSignedURLOrActiveBearer(t *testing.T) {
	db, _, bearer := accessTestDatabase(t)
	serve := func(mediaToken, authorization string) int {
		router := gin.New()
		router.GET("/musics/:id/stream", LibraryMediaAccessMiddleware(db, config.LibraryAccessAuthenticated, config.AppConfig.JWT.Secret, "stream"), func(c *gin.Context) {
			c.Status(http.StatusOK)
		})
		request := httptest.NewRequest(http.MethodGet, "/musics/42/stream?media_token="+mediaToken, nil)
		if authorization != "" {
			request.Header.Set("Authorization", authorization)
		}
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)
		return response.Code
	}

	valid := mediatoken.Issue(config.AppConfig.JWT.Secret, 42, "stream", time.Now().Add(time.Minute))
	wrongTrack := mediatoken.Issue(config.AppConfig.JWT.Secret, 41, "stream", time.Now().Add(time.Minute))
	if got := serve(valid, ""); got != http.StatusOK {
		t.Fatalf("signed media status = %d", got)
	}
	if got := serve(wrongTrack, ""); got != http.StatusUnauthorized {
		t.Fatalf("wrong-track media status = %d", got)
	}
	if got := serve("", "Bearer "+bearer); got != http.StatusOK {
		t.Fatalf("bearer fallback status = %d", got)
	}
}

func TestMusicBeeSubmitCredentialIsScopedAndRevocable(t *testing.T) {
	db, user, _ := accessTestDatabase(t)
	cfg := config.MusicBeeConfig{
		SubmitToken:    "0123456789abcdef0123456789abcdef",
		SubmitUsername: user.Username,
	}
	serve := func(token string) *httptest.ResponseRecorder {
		router := gin.New()
		router.POST("/track/submit", MusicBeeSubmitTokenMiddleware(db, cfg), func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"scope": c.GetString("credentialScope"), "user_id": c.GetUint("userID")})
		})
		request := httptest.NewRequest(http.MethodPost, "/track/submit", nil)
		if token != "" {
			request.Header.Set("Authorization", "Bearer "+token)
		}
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)
		return response
	}

	if got := serve("wrong").Code; got != http.StatusUnauthorized {
		t.Fatalf("wrong credential status = %d", got)
	}
	valid := serve(cfg.SubmitToken)
	var body struct {
		Scope  string `json:"scope"`
		UserID uint   `json:"user_id"`
	}
	if err := json.Unmarshal(valid.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode valid credential response: %v", err)
	}
	if valid.Code != http.StatusOK || body.Scope != "track:submit" || body.UserID != user.ID {
		t.Fatalf("valid credential response = %d %+v", valid.Code, body)
	}
	if err := db.Model(&domain.User{}).Where("id = ?", user.ID).Update("is_active", false).Error; err != nil {
		t.Fatalf("disable submit user: %v", err)
	}
	if got := serve(cfg.SubmitToken).Code; got != http.StatusUnauthorized {
		t.Fatalf("disabled submit user status = %d", got)
	}
}
