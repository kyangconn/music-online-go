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
	MusicTag *handler.MusicTagHandler
	Admin    *handler.AdminHandler
}

// New 创建并配置Gin路由器。
func New(h *Handlers, db *gorm.DB) (*gin.Engine, error) {
	switch config.AppConfig.Server.Mode {
	case "release":
		gin.SetMode(gin.ReleaseMode)
	case "test":
		gin.SetMode(gin.TestMode)
	default:
		gin.SetMode(gin.DebugMode)
	}

	router := gin.New()
	if err := router.SetTrustedProxies(config.AppConfig.Server.TrustedProxies); err != nil {
		return nil, fmt.Errorf("configure trusted proxies: %w", err)
	}

	if config.AppConfig.Logging.AccessLog {
		router.Use(middleware.LoggerMiddleware())
	}
	router.Use(middleware.CORSMiddleware(config.AppConfig.Server.AllowedOrigins, config.AppConfig.Server.TrustedProxies))
	if config.AppConfig.RateLimit.Enabled {
		router.Use(middleware.RateLimitMiddleware(
			rate.Limit(config.AppConfig.RateLimit.GlobalRequestsPerSecond),
			config.AppConfig.RateLimit.GlobalBurst,
		))
	}
	router.Use(middleware.JSONBodyLimitMiddleware(int64(config.AppConfig.Server.MaxJSONBodySizeMB) << 20))
	router.Use(gin.Recovery(), prometheusMiddleware())

	registerHealthAndMetrics(
		router,
		db,
		config.AppConfig.Server.UploadDir,
		time.Duration(config.AppConfig.Server.ReadinessTimeout)*time.Second,
		config.AppConfig.Metrics,
	)
	registerAPIRoutes(router, h, db, config.AppConfig.RateLimit)

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
func registerAPIRoutes(router *gin.Engine, h *Handlers, db *gorm.DB, rateLimit config.RateLimitConfig) {
	api := router.Group("/api/v1")

	registerUserRoutes(api, h.User, h.Admin, db, rateLimit)
	registerMusicRoutes(api, h.Music, db)
	registerMusicTagRoutes(api, h.MusicTag, db)
}

// registerUserRoutes 注册用户相关路由
func registerUserRoutes(api *gin.RouterGroup, userHandler *handler.UserHandler, adminHandler *handler.AdminHandler, db *gorm.DB, rateLimit config.RateLimitConfig) {
	public := api.Group("/users")
	if rateLimit.Enabled {
		public.Use(middleware.RateLimitMiddleware(rate.Limit(rateLimit.AuthRequestsPerSecond), rateLimit.AuthBurst))
	}
	{
		public.POST("/register", userHandler.Register)
		public.POST("/login", userHandler.Login)
	}

	protected := api.Group("/users")
	protected.Use(middleware.AuthMiddleware(db))
	{
		protected.GET("/profile", userHandler.GetUserProfile)
		protected.PUT("/profile", userHandler.UpdateUser)
		protected.DELETE("/profile", userHandler.DeleteUser)
		protected.POST("/change-password", userHandler.ChangePassword)

		protected.POST("/totp/setup", userHandler.SetupTOTP)
		protected.POST("/totp/enable", userHandler.EnableTOTP)
		protected.POST("/totp/disable", userHandler.DisableTOTP)

		admin := protected.Group("/admin")
		admin.Use(middleware.StrictAdminMiddleware(db))
		{
			admin.GET("/users", adminHandler.ListUsers)
			admin.PUT("/users/:id/status", adminHandler.UpdateUserStatus)
			admin.PUT("/users/:id/role", adminHandler.UpdateUserRole)
			admin.DELETE("/musics/:id", adminHandler.DeleteMusic)
			admin.GET("/system-info", adminHandler.SystemInfo)
		}
	}
}

// registerMusicRoutes 注册音乐相关路由
func registerMusicRoutes(api *gin.RouterGroup, musicHandler *handler.MusicHandler, db *gorm.DB) {
	api.GET("/upload-policy", musicHandler.UploadPolicy)

	musicPublic := api.Group("/musics")
	musicPublic.Use(middleware.OptionalAuthMiddleware())
	{
		musicPublic.GET("", musicHandler.Search)
		musicPublic.GET("/filters", musicHandler.FilterOptions)
		musicPublic.GET("/:id", musicHandler.GetByID)
	}

	api.GET("/users/:id/musics", middleware.OptionalAuthMiddleware(), musicHandler.ListUserMusic)
	api.GET("/users/:id/likes", middleware.OptionalAuthMiddleware(), musicHandler.ListUserLikedMusic)

	musicPublic.GET("/:id/stream", musicHandler.Stream)
	musicPublic.GET("/:id/cover", musicHandler.Cover)

	musicProtected := api.Group("/musics")
	musicProtected.Use(middleware.AuthMiddleware(db))
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
func registerMusicTagRoutes(api *gin.RouterGroup, musicTagHandler *handler.MusicTagHandler, db *gorm.DB) {
	musicTags := api.Group("/music-tags")
	{
		musicTags.POST("/search", musicTagHandler.SearchMusicTags)
		musicTags.GET("/:id", musicTagHandler.GetMusicTag)
		musicTags.GET("/mbid/lookup", musicTagHandler.LookupByMBID)

		musicTags.Use(middleware.AuthMiddleware(db))
		{
			musicTags.POST("", musicTagHandler.CreateMusicTag)
			musicTags.PUT("/:id", musicTagHandler.UpdateMusicTag)
			musicTags.DELETE("/:id", musicTagHandler.DeleteMusicTag)
			musicTags.POST("/match", musicTagHandler.MatchTags)
		}
	}

	tracks := api.Group("/track")
	{
		tracks.POST("/search", musicTagHandler.SearchTracks)
		tracks.POST("/submit", musicTagHandler.SubmitTrack)
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
