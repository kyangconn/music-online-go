// Package router router.go - 路由配置
// 包含中间件注册、健康检查、Prometheus指标端点及所有API路由注册
package router

import (
	"context"
	"crypto/subtle"
	"fmt"
	"net/http"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"golang.org/x/time/rate"
	"gorm.io/gorm"

	"github.com/kyangconn/music-online-go/internal/config"
	"github.com/kyangconn/music-online-go/internal/handler"
	"github.com/kyangconn/music-online-go/internal/middleware"
)

// Handlers 所有HTTP处理器的集合
type Handlers struct {
	User     *handler.UserHandler
	Music    *handler.MusicHandler
	Browse   *handler.BrowseHandler
	Playlist *handler.PlaylistHandler
	Preset   *handler.PresetClassificationHandler
	Analysis *handler.MusicAnalysisHandler
	MusicTag *handler.MusicTagHandler
	Admin    *handler.AdminHandler
}

// New creates the router from the validated startup snapshot supplied by the
// composition root.
func New(h *Handlers, db *gorm.DB, cfg config.Config) (*gin.Engine, error) {
	switch cfg.Server.Mode {
	case "release":
		gin.SetMode(gin.ReleaseMode)
	case "test":
		gin.SetMode(gin.TestMode)
	default:
		gin.SetMode(gin.DebugMode)
	}

	router := gin.New()
	if err := router.SetTrustedProxies(cfg.Server.TrustedProxies); err != nil {
		return nil, fmt.Errorf("configure trusted proxies: %w", err)
	}

	if cfg.Logging.AccessLog {
		router.Use(middleware.LoggerMiddleware())
	}
	router.Use(middleware.CORSMiddleware(cfg.Server.AllowedOrigins, cfg.Server.TrustedProxies))
	if cfg.RateLimit.Enabled {
		router.Use(middleware.RateLimitMiddleware(
			rate.Limit(cfg.RateLimit.GlobalRequestsPerSecond),
			cfg.RateLimit.GlobalBurst,
		))
	}
	router.Use(middleware.JSONBodyLimitMiddleware(int64(cfg.Server.MaxJSONBodySizeMB) << 20))
	router.Use(gin.Recovery(), prometheusMiddleware())

	registerHealthAndMetrics(
		router,
		db,
		cfg.Server.UploadDir,
		time.Duration(cfg.Server.ReadinessTimeout)*time.Second,
		cfg.Metrics,
	)
	registerAPIRoutes(router, h, db, cfg.RateLimit, cfg.Access, cfg.Classification, cfg.Integrations, cfg.JWT.Secret)

	return router, nil
}

// registerHealthAndMetrics 注册健康检查和Prometheus指标端点
func registerHealthAndMetrics(router *gin.Engine, db *gorm.DB, uploadDir string, readinessTimeout time.Duration, metrics config.MetricsConfig) {
	// /health 存活检查：仅确认进程运行中
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":  "ok",
			"time":    time.Now().Format(time.RFC3339),
			"version": runtime.Version(),
		})
	})

	// /ready 就绪检查：确认数据库和上传存储均可用
	router.GET("/ready", func(c *gin.Context) {
		if db == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"status": "not ready",
				"error":  "database handle unavailable",
			})
			return
		}
		sqlDB, err := db.DB()
		if err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"status": "not ready",
				"error":  "database handle unavailable",
			})
			return
		}
		ctx, cancel := context.WithTimeout(c.Request.Context(), readinessTimeout)
		defer cancel()
		if err := sqlDB.PingContext(ctx); err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"status": "not ready",
				"error":  fmt.Sprintf("database ping failed: %v", err),
			})
			return
		}
		if err := checkUploadStorage(uploadDir); err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"status": "not ready",
				"error":  "upload storage unavailable",
			})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"status": "ready",
			"time":   time.Now().Format(time.RFC3339),
		})
	})

	metricsHandler := gin.WrapH(promhttp.Handler())
	router.GET("/metrics", func(c *gin.Context) {
		if !metrics.Enabled {
			c.Status(http.StatusNotFound)
			return
		}
		if !hasMetricsToken(c.GetHeader("Authorization"), metrics.Token) {
			c.Header("WWW-Authenticate", "Bearer")
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}
		metricsHandler(c)
	})
}

func checkUploadStorage(uploadDir string) error {
	if strings.TrimSpace(uploadDir) == "" {
		return fmt.Errorf("upload directory is empty")
	}
	if err := os.MkdirAll(uploadDir, 0700); err != nil {
		return fmt.Errorf("create upload directory: %w", err)
	}
	probe, err := os.CreateTemp(uploadDir, ".ready-*.tmp")
	if err != nil {
		return fmt.Errorf("create storage probe: %w", err)
	}
	probePath := probe.Name()
	defer func() { _ = os.Remove(probePath) }()
	if err := probe.Close(); err != nil {
		return fmt.Errorf("close storage probe: %w", err)
	}
	if err := os.Remove(probePath); err != nil {
		return fmt.Errorf("remove storage probe: %w", err)
	}
	return nil
}

func hasMetricsToken(header, expected string) bool {
	const prefix = "Bearer "
	if !strings.HasPrefix(header, prefix) {
		return false
	}
	provided := strings.TrimPrefix(header, prefix)
	return subtle.ConstantTimeCompare([]byte(provided), []byte(expected)) == 1
}

// registerAPIRoutes 注册所有API路由
func registerAPIRoutes(router *gin.Engine, h *Handlers, db *gorm.DB, rateLimit config.RateLimitConfig, access config.AccessConfig, classification config.ClassificationConfig, integrations config.IntegrationsConfig, jwtSecret string) {
	api := router.Group("/api/v1")
	api.GET("/instance", func(c *gin.Context) {
		handler.Success(c, gin.H{
			"library_mode":            access.LibraryMode,
			"registration_mode":       access.RegistrationMode,
			"registration_open":       access.RegistrationMode == config.RegistrationOpen,
			"musicbee_submit_enabled": strings.TrimSpace(integrations.MusicBee.SubmitToken) != "",
			"classification_enabled":  classification.Enabled,
			"audio_analyzer_enabled":  classification.Enabled && classification.Analyzer.Mode == "http",
			"analyze_on_upload":       classification.Enabled && classification.AnalyzeOnUpload,
		})
	})

	registerUserRoutes(api, h.User, h.Admin, h.Preset, h.Analysis, db, rateLimit, access, jwtSecret)
	registerMusicRoutes(api, h.Music, db, access, jwtSecret)
	registerBrowseRoutes(api, h.Browse, h.Preset, db, access, jwtSecret)
	registerPlaylistRoutes(api, h.Playlist, db, jwtSecret)
	registerMusicTagRoutes(api, h.MusicTag, db, access, jwtSecret, integrations.MusicBee)
}

func registerBrowseRoutes(
	api *gin.RouterGroup,
	browseHandler *handler.BrowseHandler,
	presetHandler *handler.PresetClassificationHandler,
	db *gorm.DB,
	access config.AccessConfig,
	jwtSecret string,
) {
	browse := api.Group("")
	browse.Use(middleware.LibraryReadMiddleware(db, access.LibraryMode, jwtSecret))
	{
		browse.GET("/artists", browseHandler.ListArtists)
		browse.GET("/artists/:key", browseHandler.GetArtist)
		browse.GET("/albums", browseHandler.ListAlbums)
		browse.GET("/albums/:key", browseHandler.GetAlbum)
		if presetHandler != nil {
			browse.GET("/presets", presetHandler.ListPresets)
		}
	}
}

func registerPlaylistRoutes(api *gin.RouterGroup, playlistHandler *handler.PlaylistHandler, db *gorm.DB, jwtSecret string) {
	playlists := api.Group("/playlists")
	playlists.Use(middleware.AuthMiddleware(db, jwtSecret))
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
}

// registerUserRoutes 注册用户相关路由
func registerUserRoutes(
	api *gin.RouterGroup,
	userHandler *handler.UserHandler,
	adminHandler *handler.AdminHandler,
	presetHandler *handler.PresetClassificationHandler,
	analysisHandler *handler.MusicAnalysisHandler,
	db *gorm.DB,
	rateLimit config.RateLimitConfig,
	access config.AccessConfig,
	jwtSecret string,
) {
	public := api.Group("/users")
	if rateLimit.Enabled {
		public.Use(middleware.RateLimitMiddleware(rate.Limit(rateLimit.AuthRequestsPerSecond), rateLimit.AuthBurst))
	}
	{
		public.POST("/register", func(c *gin.Context) {
			if access.RegistrationMode != config.RegistrationOpen {
				handler.Forbidden(c, "Public registration is disabled")
				return
			}
			userHandler.Register(c)
		})
		public.POST("/login", userHandler.Login)
		public.POST("/refresh", userHandler.Refresh)
		// logout 使用可选认证：access token 过期时仍可通过 refresh cookie 撤销会话。
		public.POST("/logout", middleware.OptionalAuthMiddleware(db, jwtSecret), userHandler.Logout)
	}

	protected := api.Group("/users")
	protected.Use(middleware.AuthMiddleware(db, jwtSecret))
	{
		protected.GET("/profile", userHandler.GetUserProfile)
		protected.PUT("/profile", userHandler.UpdateUser)
		protected.DELETE("/profile", userHandler.DeleteUser)
		protected.POST("/change-password", userHandler.ChangePassword)
		protected.POST("/logout-all", userHandler.LogoutAll)

		protected.POST("/totp/setup", userHandler.SetupTOTP)
		protected.POST("/totp/enable", userHandler.EnableTOTP)
		protected.POST("/totp/disable", userHandler.DisableTOTP)

		admin := protected.Group("/admin")
		admin.Use(middleware.StrictAdminMiddleware(db))
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
			admin.POST("/media-library/roots/:id/probe", adminHandler.ProbeMediaLibraryRoot)
			admin.POST("/media-library/roots/:id/scans", adminHandler.StartMediaLibraryScan)
			admin.GET("/media-library/scans", adminHandler.ListMediaLibraryScans)
			admin.GET("/media-library/scans/:id", adminHandler.GetMediaLibraryScan)
			admin.POST("/media-library/scans/:id/cancel", adminHandler.CancelMediaLibraryScan)
			if presetHandler != nil {
				admin.POST("/classifications/manual-batch", presetHandler.SetManualPresets)
				admin.POST("/musics/:id/classification/reclassify", presetHandler.Reclassify)
				admin.PUT("/musics/:id/classification/manual", presetHandler.SetManualPreset)
				admin.DELETE("/musics/:id/classification/manual", presetHandler.ClearManualPreset)
			}
			if analysisHandler != nil {
				admin.POST("/musics/:id/analysis", analysisHandler.ScheduleMusic)
				admin.POST("/analysis/backfill", analysisHandler.Backfill)
				admin.GET("/analysis/jobs", analysisHandler.ListJobs)
				admin.GET("/analysis/jobs/:id", analysisHandler.GetJob)
				admin.POST("/analysis/jobs/:id/cancel", analysisHandler.CancelJob)
				admin.GET("/analysis/metrics", analysisHandler.Metrics)
			}
		}
	}
}

// registerMusicRoutes 注册音乐相关路由
func registerMusicRoutes(api *gin.RouterGroup, musicHandler *handler.MusicHandler, db *gorm.DB, access config.AccessConfig, jwtSecret string) {
	api.GET("/upload-policy", musicHandler.UploadPolicy)

	musicPublic := api.Group("/musics")
	musicPublic.Use(middleware.LibraryReadMiddleware(db, access.LibraryMode, jwtSecret))
	{
		musicPublic.GET("", musicHandler.Search)
		musicPublic.GET("/filters", musicHandler.FilterOptions)
		musicPublic.GET("/:id", musicHandler.GetByID)
	}

	api.GET("/users/:id/musics", middleware.LibraryReadMiddleware(db, access.LibraryMode, jwtSecret), musicHandler.ListUserMusic)
	api.GET("/users/:id/likes", middleware.LibraryReadMiddleware(db, access.LibraryMode, jwtSecret), musicHandler.ListUserLikedMusic)

	media := api.Group("/musics")
	media.GET("/:id/stream", middleware.LibraryMediaAccessMiddleware(db, access.LibraryMode, jwtSecret, jwtSecret, "stream"), musicHandler.Stream)
	media.GET("/:id/cover", middleware.LibraryMediaAccessMiddleware(db, access.LibraryMode, jwtSecret, jwtSecret, "cover"), musicHandler.Cover)

	musicProtected := api.Group("/musics")
	musicProtected.Use(middleware.AuthMiddleware(db, jwtSecret))
	{
		musicProtected.POST("", musicHandler.Create)
		musicProtected.POST("/duplicate-check", musicHandler.CheckDuplicates)
		musicProtected.POST("/:id/upload", musicHandler.UploadFile)
		musicProtected.PUT("/:id", musicHandler.Update)
		musicProtected.DELETE("/:id", musicHandler.Delete)
		musicProtected.POST("/:id/like", musicHandler.Like)
		musicProtected.DELETE("/:id/like", musicHandler.Unlike)
	}
}

// registerMusicTagRoutes 注册音乐标签相关路由
func registerMusicTagRoutes(api *gin.RouterGroup, musicTagHandler *handler.MusicTagHandler, db *gorm.DB, access config.AccessConfig, jwtSecret string, musicBee config.MusicBeeConfig) {
	musicTagReads := api.Group("/music-tags")
	musicTagReads.Use(middleware.LibraryReadMiddleware(db, access.LibraryMode, jwtSecret))
	{
		musicTagReads.POST("/search", musicTagHandler.SearchMusicTags)
		musicTagReads.POST("/match", musicTagHandler.MatchTags)
		musicTagReads.GET("/mbid/lookup", musicTagHandler.LookupByMBID)
	}

	tracks := api.Group("/track")
	{
		tracks.POST("/search", middleware.LibraryReadMiddleware(db, access.LibraryMode, jwtSecret), musicTagHandler.SearchTracks)
		if strings.TrimSpace(musicBee.SubmitToken) != "" {
			tracks.POST("/submit", middleware.MusicBeeSubmitTokenMiddleware(db, musicBee), musicTagHandler.SubmitTrack)
		}
	}
}

// prometheusMetrics Prometheus 自定义指标
var (
	httpRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "HTTP 请求总数",
		},
		[]string{"method", "path", "status"},
	)
	httpRequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "HTTP 请求耗时（秒）",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "path"},
	)
)

func init() {
	prometheus.MustRegister(httpRequestsTotal, httpRequestDuration)
}

func prometheusMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.FullPath()
		if path == "" {
			path = "unknown"
		}

		c.Next()

		status := fmt.Sprintf("%d", c.Writer.Status())
		httpRequestsTotal.WithLabelValues(c.Request.Method, path, status).Inc()
		httpRequestDuration.WithLabelValues(c.Request.Method, path).Observe(time.Since(start).Seconds())
	}
}
