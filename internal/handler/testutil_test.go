package handler_test

import (
	"os"
	"testing"

	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"

	"github.com/kyangconn/music-online-go/internal/config"
	"github.com/kyangconn/music-online-go/internal/handler"
	"github.com/kyangconn/music-online-go/internal/middleware"
	"github.com/kyangconn/music-online-go/internal/pkg/database"
	"github.com/kyangconn/music-online-go/internal/repository"
	"github.com/kyangconn/music-online-go/internal/service"
)

var testRouter *gin.Engine

func TestMain(m *testing.M) {
	config.AppConfig = &config.Config{
		Database: config.DatabaseConfig{Type: "sqlite", Path: ":memory:"},
		Server:   config.ServerConfig{UploadDir: os.TempDir()},
		JWT:      config.JWTConfig{Secret: "test-secret", ExpireHour: 24},
	}

	gin.SetMode(gin.TestMode)

	if err := database.Connect(); err != nil {
		panic(err)
	}
	if err := database.AutoMigrate(); err != nil {
		panic(err)
	}

	userRepo := repository.NewUserRepository(database.DB)
	userService := service.NewUserService(userRepo)

	musicRepo := repository.NewMusicRepository(database.DB)
	musicService := service.NewMusicService(musicRepo)

	musicTagRepo := repository.NewMusicTagRepository(database.DB)
	musicTagService := service.NewMusicTagService(musicTagRepo)

	userHandler := handler.NewUserHandler(userService)
	musicHandler := handler.NewMusicHandler(musicService)
	musicTagHandler := handler.NewMusicTagHandler(musicTagService)
	adminHandler := handler.NewAdminHandler(userService, musicService, musicTagService)

	testRouter = gin.New()
	testRouter.Use(gin.Recovery())

	testRouter.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	api := testRouter.Group("/api/v1")

	public := api.Group("/users")
	public.Use(middleware.RateLimitMiddleware(rate.Limit(100), 100))
	{
		public.POST("/register", userHandler.Register)
		public.POST("/login", userHandler.Login)
	}

	protected := api.Group("/users")
	protected.Use(middleware.AuthMiddleware())
	{
		protected.GET("/profile", userHandler.GetUserProfile)
		protected.PUT("/profile", userHandler.UpdateUser)
		protected.DELETE("/profile", userHandler.DeleteUser)
		protected.POST("/change-password", userHandler.ChangePassword)
		protected.POST("/totp/setup", userHandler.SetupTOTP)
		protected.POST("/totp/enable", userHandler.EnableTOTP)
		protected.POST("/totp/disable", userHandler.DisableTOTP)

		admin := protected.Group("/admin")
		admin.Use(middleware.StrictAdminMiddleware(database.DB))
		{
			admin.GET("/users", adminHandler.ListUsers)
			admin.PUT("/users/:id/status", adminHandler.UpdateUserStatus)
			admin.PUT("/users/:id/role", adminHandler.UpdateUserRole)
			admin.DELETE("/musics/:id", adminHandler.DeleteMusic)
			admin.GET("/system-info", adminHandler.SystemInfo)
		}
	}

	musicPublic := api.Group("/musics")
	musicPublic.Use(middleware.OptionalAuthMiddleware())
	{
		musicPublic.GET("", musicHandler.Search)
		musicPublic.GET("/:id", musicHandler.GetByID)
	}
	api.GET("/users/:id/musics", middleware.OptionalAuthMiddleware(), musicHandler.ListUserMusic)
	api.GET("/users/:id/likes", middleware.OptionalAuthMiddleware(), musicHandler.ListUserLikedMusic)

	musicProtected := api.Group("/musics")
	musicProtected.Use(middleware.AuthMiddleware())
	{
		musicProtected.POST("", musicHandler.Create)
		musicProtected.POST("/:id/upload", musicHandler.UploadFile)
		musicProtected.PUT("/:id", musicHandler.Update)
		musicProtected.DELETE("/:id", musicHandler.Delete)
		musicProtected.POST("/:id/like", musicHandler.Like)
		musicProtected.DELETE("/:id/like", musicHandler.Unlike)
	}

	musicTags := api.Group("/music-tags")
	{
		musicTags.POST("/search", musicTagHandler.SearchMusicTags)
		musicTags.GET("/:id", musicTagHandler.GetMusicTag)
		musicTags.GET("/mbid/lookup", musicTagHandler.LookupByMBID)

		musicTags.Use(middleware.AuthMiddleware())
		{
			musicTags.POST("", musicTagHandler.CreateMusicTag)
			musicTags.PUT("/:id", musicTagHandler.UpdateMusicTag)
			musicTags.DELETE("/:id", musicTagHandler.DeleteMusicTag)
			musicTags.POST("/match", musicTagHandler.MatchTags)
		}
	}

	code := m.Run()
	database.Close()
	os.Exit(code)
}
