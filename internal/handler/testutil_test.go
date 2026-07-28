// Package handler_test testutil_test.go - 测试工具
// 负责初始化测试环境（数据库、路由、处理器），供集成测试使用
package handler_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"

	"github.com/kyangconn/music-online-go/internal/config"
	"github.com/kyangconn/music-online-go/internal/domain"
	"github.com/kyangconn/music-online-go/internal/handler"
	"github.com/kyangconn/music-online-go/internal/middleware"
	"github.com/kyangconn/music-online-go/internal/pkg/database"
	"github.com/kyangconn/music-online-go/internal/repository"
	"github.com/kyangconn/music-online-go/internal/service"
)

var testRouter *gin.Engine
var testUploadDir string

func TestMain(m *testing.M) {
	var err error
	testUploadDir, err = os.MkdirTemp("", "music-online-handler-test-")
	if err != nil {
		panic(err)
	}
	config.AppConfig = &config.Config{
		Database: config.DatabaseConfig{
			Type:                         "sqlite",
			Path:                         ":memory:",
			LogLevel:                     "auto",
			ConnectTimeoutSeconds:        10,
			ConnectionMaxLifetimeMinutes: 60,
			ConnectionMaxIdleTimeMinutes: 10,
		},
		Server: config.ServerConfig{UploadDir: testUploadDir, MaxAudioSizeMB: 1, MaxCoverSizeMB: 1},
		JWT:    config.JWTConfig{Secret: "test-secret", ExpireHour: 24},
		Library: config.LibraryConfig{Scanner: config.LibraryScannerConfig{
			Enabled: true, MaxFileSizeMB: 1,
		}},
		Classification: config.ClassificationConfig{
			Enabled: true, AutoThreshold: 0.65, ReviewMargin: 0.12,
			CalmFlowWeight: 1, KineticPulseWeight: 1, CosmicDriftWeight: 1, BassImpactWeight: 1,
		},
	}

	gin.SetMode(gin.TestMode)

	if err := database.Connect(); err != nil {
		panic(err)
	}
	if err := database.Migrate(); err != nil {
		panic(err)
	}

	userRepo := repository.NewUserRepository(database.DB)
	userService := service.NewUserService(userRepo)

	presetPolicy := domain.DefaultPresetRulePolicy()
	musicRepo := repository.NewMusicRepository(database.DB, presetPolicy)
	browseRepo := repository.NewBrowseRepository(database.DB)
	playlistRepo := repository.NewPlaylistRepository(database.DB)
	presetRepo := repository.NewPresetRepository(database.DB, presetPolicy)
	analysisRepo := repository.NewMusicAnalysisRepository(database.DB)
	mediaLibraryRepo := repository.NewMediaLibraryRepository(database.DB, presetPolicy)
	mediaLibraryService := service.NewMediaLibraryService(mediaLibraryRepo, musicRepo, config.AppConfig.Library)
	analysisService := service.NewMusicAnalysisService(analysisRepo, presetRepo, mediaLibraryService, config.AppConfig.Classification)
	mediaLibraryService.SetAnalysisScheduler(analysisService)
	musicService := service.NewMusicServiceWithAnalysis(musicRepo, mediaLibraryService, config.AppConfig, presetRepo, analysisRepo)
	musicService.SetAnalysisScheduler(analysisService)
	browseService := service.NewBrowseService(browseRepo, config.AppConfig)
	playlistService := service.NewPlaylistService(playlistRepo, musicService)
	presetService := service.NewPresetClassificationService(presetRepo, true)

	userHandler := handler.NewUserHandler(userService)
	musicHandler := handler.NewMusicHandler(musicService)
	browseHandler := handler.NewBrowseHandler(browseService)
	playlistHandler := handler.NewPlaylistHandler(playlistService)
	presetHandler := handler.NewPresetClassificationHandler(presetService)
	analysisHandler := handler.NewMusicAnalysisHandler(analysisService)
	musicTagHandler := handler.NewMusicTagHandler(musicService)
	adminHandler := handler.NewAdminHandler(userService, musicService, mediaLibraryService)

	testRouter = gin.New()
	testRouter.Use(gin.Recovery())

	testRouter.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	testRouter.GET("/ready", func(c *gin.Context) {
		sqlDB, err := database.DB.DB()
		if err != nil {
			c.JSON(503, gin.H{"status": "not ready"})
			return
		}
		if err := sqlDB.Ping(); err != nil {
			c.JSON(503, gin.H{"status": "not ready"})
			return
		}
		c.JSON(200, gin.H{"status": "ready"})
	})

	api := testRouter.Group("/api/v1")

	public := api.Group("/users")
	public.Use(middleware.RateLimitMiddleware(rate.Limit(100), 100))
	{
		public.POST("/register", userHandler.Register)
		public.POST("/login", userHandler.Login)
	}

	protected := api.Group("/users")
	protected.Use(middleware.AuthMiddleware(database.DB))
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
			admin.POST("/users", adminHandler.CreateUser)
			admin.PUT("/users/:id/status", adminHandler.UpdateUserStatus)
			admin.PUT("/users/:id/role", adminHandler.UpdateUserRole)
			admin.DELETE("/musics/:id", adminHandler.DeleteMusic)
			admin.GET("/system-info", adminHandler.SystemInfo)
			admin.GET("/media-library/roots", adminHandler.ListMediaLibraryRoots)
			admin.POST("/media-library/roots", adminHandler.CreateMediaLibraryRoot)
			admin.PATCH("/media-library/roots/:id", adminHandler.UpdateMediaLibraryRoot)
			admin.DELETE("/media-library/roots/:id", adminHandler.DeleteMediaLibraryRoot)
			admin.POST("/media-library/roots/:id/scans", adminHandler.StartMediaLibraryScan)
			admin.GET("/media-library/scans", adminHandler.ListMediaLibraryScans)
			admin.GET("/media-library/scans/:id", adminHandler.GetMediaLibraryScan)
			admin.POST("/media-library/scans/:id/cancel", adminHandler.CancelMediaLibraryScan)
			admin.POST("/musics/:id/classification/reclassify", presetHandler.Reclassify)
			admin.PUT("/musics/:id/classification/manual", presetHandler.SetManualPreset)
			admin.DELETE("/musics/:id/classification/manual", presetHandler.ClearManualPreset)
			admin.POST("/musics/:id/analysis", analysisHandler.ScheduleMusic)
			admin.POST("/analysis/backfill", analysisHandler.Backfill)
			admin.GET("/analysis/jobs", analysisHandler.ListJobs)
			admin.GET("/analysis/jobs/:id", analysisHandler.GetJob)
			admin.POST("/analysis/jobs/:id/cancel", analysisHandler.CancelJob)
			admin.GET("/analysis/metrics", analysisHandler.Metrics)
		}
	}

	musicPublic := api.Group("/musics")
	musicPublic.Use(middleware.OptionalAuthMiddleware())
	{
		musicPublic.GET("", musicHandler.Search)
		musicPublic.GET("/filters", musicHandler.FilterOptions)
		musicPublic.GET("/:id", musicHandler.GetByID)
		musicPublic.GET("/:id/stream", musicHandler.Stream)
		musicPublic.GET("/:id/cover", musicHandler.Cover)
	}
	api.GET("/upload-policy", musicHandler.UploadPolicy)
	api.GET("/users/:id/musics", middleware.OptionalAuthMiddleware(), musicHandler.ListUserMusic)
	api.GET("/users/:id/likes", middleware.OptionalAuthMiddleware(), musicHandler.ListUserLikedMusic)

	browse := api.Group("")
	browse.Use(middleware.OptionalAuthMiddleware())
	{
		browse.GET("/artists", browseHandler.ListArtists)
		browse.GET("/artists/:key", browseHandler.GetArtist)
		browse.GET("/albums", browseHandler.ListAlbums)
		browse.GET("/albums/:key", browseHandler.GetAlbum)
	}

	playlists := api.Group("/playlists")
	playlists.Use(middleware.AuthMiddleware(database.DB))
	{
		playlists.GET("", playlistHandler.List)
		playlists.POST("", playlistHandler.Create)
		playlists.GET("/:id", playlistHandler.Get)
		playlists.PATCH("/:id", playlistHandler.Update)
		playlists.DELETE("/:id", playlistHandler.Delete)
		playlists.POST("/:id/items", playlistHandler.AddItem)
		playlists.PUT("/:id/items/order", playlistHandler.ReorderItems)
		playlists.DELETE("/:id/items/:musicID", playlistHandler.RemoveItem)
	}

	musicProtected := api.Group("/musics")
	musicProtected.Use(middleware.AuthMiddleware(database.DB))
	{
		musicProtected.POST("", musicHandler.Create)
		musicProtected.POST("/duplicate-check", musicHandler.CheckDuplicates)
		musicProtected.POST("/:id/upload", musicHandler.UploadFile)
		musicProtected.PUT("/:id", musicHandler.Update)
		musicProtected.DELETE("/:id", musicHandler.Delete)
		musicProtected.POST("/:id/like", musicHandler.Like)
		musicProtected.DELETE("/:id/like", musicHandler.Unlike)
	}

	musicTagReads := api.Group("/music-tags")
	musicTagReads.Use(middleware.OptionalAuthMiddleware())
	{
		musicTagReads.POST("/search", musicTagHandler.SearchMusicTags)
		musicTagReads.POST("/match", musicTagHandler.MatchTags)
		musicTagReads.GET("/mbid/lookup", musicTagHandler.LookupByMBID)
	}

	code := m.Run()
	if err := database.Close(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to close test database: %v\n", err)
		os.Exit(1)
	}
	if err := os.RemoveAll(testUploadDir); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to remove test uploads: %v\n", err)
		os.Exit(1)
	}
	os.Exit(code)
}
