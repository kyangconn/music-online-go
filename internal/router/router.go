// Package router router.go - 路由配置
// 包含中间件注册、健康检查、Prometheus指标端点及所有API路由注册
package router

import (
	"fmt"
	"net/http"
	"runtime"
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

// New 创建并配置Gin路由器
func New(h *Handlers, db *gorm.DB) *gin.Engine {
	if config.AppConfig.Server.Mode == "release" {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.New()

	router.Use(
		middleware.LoggerMiddleware(),
		middleware.CORSMiddleware(),
		middleware.RateLimitMiddleware(rate.Limit(20), 50),
		gin.Recovery(),
		prometheusMiddleware(),
	)

	registerHealthAndMetrics(router)
	registerAPIRoutes(router, h, db)

	return router
}

// registerHealthAndMetrics 注册健康检查和Prometheus指标端点
func registerHealthAndMetrics(router *gin.Engine) {
	router.GET("/health", func(c *gin.Context) {
		// 注意：health endpoint 无法访问 DB，由调用方注入后此处可扩展
		c.JSON(http.StatusOK, gin.H{
			"status":  "ok",
			"time":    time.Now().Format(time.RFC3339),
			"version": runtime.Version(),
		})
	})

	router.GET("/metrics", gin.WrapH(promhttp.Handler()))
}

// registerAPIRoutes 注册所有API路由
func registerAPIRoutes(router *gin.Engine, h *Handlers, db *gorm.DB) {
	api := router.Group("/api/v1")

	registerUserRoutes(api, h.User, h.Admin, db)
	registerMusicRoutes(api, h.Music)
	registerMusicTagRoutes(api, h.MusicTag)
}

// registerUserRoutes 注册用户相关路由
func registerUserRoutes(api *gin.RouterGroup, userHandler *handler.UserHandler, adminHandler *handler.AdminHandler, db *gorm.DB) {
	public := api.Group("/users")
	public.Use(middleware.RateLimitMiddleware(rate.Limit(1), 5))
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
func registerMusicRoutes(api *gin.RouterGroup, musicHandler *handler.MusicHandler) {
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
	musicProtected.Use(middleware.AuthMiddleware())
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
func registerMusicTagRoutes(api *gin.RouterGroup, musicTagHandler *handler.MusicTagHandler) {
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
