// main.go - 音乐在线平台服务器主程序
// 该文件包含应用程序的入口点，负责初始化配置、数据库连接、依赖注入、路由设置和服务器启动。
package main

import (
	"context"
	"embed"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"

	"github.com/kyangconn/music-online-go/internal/config"
	"github.com/kyangconn/music-online-go/internal/handler"
	"github.com/kyangconn/music-online-go/internal/middleware"
	"github.com/kyangconn/music-online-go/internal/pkg/database"
	"github.com/kyangconn/music-online-go/internal/repository"
	"github.com/kyangconn/music-online-go/internal/service"
)

// webDist 嵌入前端构建产物
// 使用Go 1.16+的embed功能将Vue前端构建产物嵌入到Go二进制文件中
// 实现单文件分发，无需单独部署前端静态文件
//
//go:embed dist/*
var webDist embed.FS

func main() {
	// 1. 初始化配置和数据库
	initConfigAndDatabase()

	// 2. 初始化依赖
	handlers := initDependencies()

	// 3. 创建并配置路由器
	router := createRouter(handlers)

	// 4. 配置静态资源托管
	configureStaticAssets(router)

	// 5. 启动服务器
	startServer(router)
}

// initConfigAndDatabase 初始化配置和数据库连接
func initConfigAndDatabase() {
	// 加载配置
	if err := config.LoadConfig(); err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// 连接数据库
	if err := database.Connect(); err != nil {
		log.Fatalf("Failed to connect database: %v", err)
	}
	defer database.Close()

	// 自动迁移表结构
	if err := database.AutoMigrate(); err != nil {
		log.Fatalf("Failed to migrate database: %v", err)
	}
}

// initDependencies 初始化所有依赖项
func initDependencies() *AppHandlers {
	userRepo := repository.NewUserRepository(database.DB)
	userService := service.NewUserService(userRepo)
	userHandler := handler.NewUserHandler(userService)

	musicRepo := repository.NewMusicRepository(database.DB)
	musicService := service.NewMusicService(musicRepo)
	musicHandler := handler.NewMusicHandler(musicService)

	// Music Tag module
	musicTagRepo := repository.NewMusicTagRepository(database.DB)
	musicTagService := service.NewMusicTagService(musicTagRepo)
	musicTagHandler := handler.NewMusicTagHandler(musicTagService)

	adminHandler := handler.NewAdminHandler(userService, musicService)

	return &AppHandlers{
		userHandler:     userHandler,
		musicHandler:    musicHandler,
		musicTagHandler: musicTagHandler,
		adminHandler:    adminHandler,
	}
}

// AppHandlers 应用处理器集合
type AppHandlers struct {
	userHandler     *handler.UserHandler
	musicHandler    *handler.MusicHandler
	musicTagHandler *handler.MusicTagHandler
	adminHandler    *handler.AdminHandler
}

// createRouter 创建并配置路由器
func createRouter(handlers *AppHandlers) *gin.Engine {
	// 设置Gin模式
	if config.AppConfig.Server.Mode == "release" {
		gin.SetMode(gin.ReleaseMode)
	}

	// 创建Gin引擎
	router := gin.New()

	// 注册中间件
	registerMiddleware(router)

	// 注册健康检查路由
	registerHealthCheckRoute(router)

	// 注册API路由
	registerAPIRoutes(router, handlers)

	return router
}

// registerMiddleware 注册中间件
func registerMiddleware(router *gin.Engine) {
	router.Use(
		middleware.LoggerMiddleware(),
		middleware.CORSMiddleware(),
		middleware.RateLimitMiddleware(rate.Limit(20), 50), // 全局限流：每秒20请求，突发50
		gin.Recovery(),
	)
}

// registerHealthCheckRoute 注册健康检查路由
func registerHealthCheckRoute(router *gin.Engine) {
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status": "ok",
			"time":   time.Now().Format(time.RFC3339),
		})
	})
}

// registerAPIRoutes 注册API路由
func registerAPIRoutes(router *gin.Engine, handlers *AppHandlers) {
	api := router.Group("/api/v1")

	// 注册用户相关路由
	registerUserRoutes(api, handlers.userHandler, handlers.adminHandler)

	// 注册音乐相关路由
	registerMusicRoutes(api, handlers.musicHandler)

	// 注册音乐标签相关路由
	registerMusicTagRoutes(api, handlers.musicTagHandler)
}

// registerUserRoutes 注册用户相关路由
func registerUserRoutes(api *gin.RouterGroup, userHandler *handler.UserHandler, adminHandler *handler.AdminHandler) {
	// 公开路由
	public := api.Group("/users")
	// 登录注册限流：每秒1请求，突发5
	public.Use(middleware.RateLimitMiddleware(rate.Limit(1), 5))
	{
		public.POST("/register", userHandler.Register)
		public.POST("/login", userHandler.Login)
	}

	// 需要认证的路由
	protected := api.Group("/users")
	protected.Use(middleware.AuthMiddleware())
	{
		protected.GET("/profile", userHandler.GetUserProfile)
		protected.PUT("/profile", userHandler.UpdateUser)
		protected.DELETE("/profile", userHandler.DeleteUser)
		protected.POST("/change-password", userHandler.ChangePassword)

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
}

// registerMusicRoutes 注册音乐相关路由
func registerMusicRoutes(api *gin.RouterGroup, musicHandler *handler.MusicHandler) {
	// 公开/可选认证
	musicPublic := api.Group("/musics")
	musicPublic.Use(middleware.OptionalAuthMiddleware())
	{
		musicPublic.GET("", musicHandler.Search)
		musicPublic.GET("/:id", musicHandler.GetByID)
	}

	api.GET("/users/:id/musics", middleware.OptionalAuthMiddleware(), musicHandler.ListUserMusic)
	api.GET("/users/:id/likes", middleware.OptionalAuthMiddleware(), musicHandler.ListUserLikedMusic)

	// 需要认证
	musicProtected := api.Group("/musics")
	musicProtected.Use(middleware.AuthMiddleware())
	{
		musicProtected.POST("", musicHandler.Create)
		musicProtected.PUT("/:id", musicHandler.Update)
		musicProtected.DELETE("/:id", musicHandler.Delete)
		musicProtected.POST("/:id/like", musicHandler.Like)
		musicProtected.DELETE("/:id/like", musicHandler.Unlike)
	}
}

// registerMusicTagRoutes 注册音乐标签相关路由
func registerMusicTagRoutes(api *gin.RouterGroup, musicTagHandler *handler.MusicTagHandler) {
	// Music Tag module (MusicBee compatible API)
	musicTags := api.Group("/music-tags")
	{
		// 公开端点
		musicTags.POST("/search", musicTagHandler.SearchMusicTags)
		musicTags.GET("/:id", musicTagHandler.GetMusicTag)
		musicTags.GET("/mbid/lookup", musicTagHandler.LookupByMBID)

		// 需要认证的端点
		musicTags.Use(middleware.AuthMiddleware())
		{
			musicTags.POST("", musicTagHandler.CreateMusicTag)
			musicTags.PUT("/:id", musicTagHandler.UpdateMusicTag)
			musicTags.DELETE("/:id", musicTagHandler.DeleteMusicTag)
			musicTags.POST("/match", musicTagHandler.MatchTags)
		}
	}

	// MusicBee兼容的track端点
	tracks := api.Group("/track")
	{
		tracks.POST("/search", musicTagHandler.SearchTracks)
		tracks.POST("/submit", musicTagHandler.SubmitTrack)
	}
}

// configureStaticAssets 配置静态资源托管
func configureStaticAssets(router *gin.Engine) {
	// 获取 dist 子目录文件系统
	distFS, err := fs.Sub(webDist, "dist")
	if err != nil {
		log.Fatalf("Failed to create dist fs: %v", err)
	}
	httpFS := http.FS(distFS)

	router.NoRoute(func(c *gin.Context) {
		handleSPARouting(c, distFS, httpFS)
	})
}

// handleSPARouting 处理SPA路由
func handleSPARouting(c *gin.Context, distFS fs.FS, httpFS http.FileSystem) {
	// 1. 如果是 API 请求，返回 404 JSON
	if strings.HasPrefix(c.Request.URL.Path, "/api") {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}

	// 2. 尝试查找静态文件
	path := strings.TrimPrefix(c.Request.URL.Path, "/")
	if path == "" {
		path = "index.html"
	}

	// 检查文件是否存在
	if _, err := distFS.Open(path); err != nil {
		// 文件不存在，返回 index.html (SPA History 模式支持)
		c.FileFromFS("index.html", httpFS)
		return
	}

	// 文件存在，直接返回
	c.FileFromFS(path, httpFS)
}

// startServer 启动服务器
func startServer(router *gin.Engine) {
	// 获取端口配置
	port := getServerPort()

	// 创建HTTP服务器
	srv := createHTTPServer(router, port)

	// 启动服务器
	startHTTPServer(srv, port)

	// 等待优雅关机
	waitForShutdown(srv)
}

// getServerPort 获取服务器端口
func getServerPort() string {
	port := config.AppConfig.Server.Port
	if port == "" {
		port = "8080"
	}
	return port
}

// createHTTPServer 创建HTTP服务器
func createHTTPServer(router *gin.Engine, port string) *http.Server {
	return &http.Server{
		Addr:         ":" + port,
		Handler:      router,
		ReadTimeout:  time.Duration(config.AppConfig.Server.ReadTimeout) * time.Second,
		WriteTimeout: time.Duration(config.AppConfig.Server.WriteTimeout) * time.Second,
	}
}

// startHTTPServer 启动HTTP服务器
func startHTTPServer(srv *http.Server, port string) {
	go func() {
		log.Printf("Server starting on port %s", port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()
}

// waitForShutdown 等待优雅关机
func waitForShutdown(srv *http.Server) {
	// 等待中断信号
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatal("Server forced to shutdown:", err)
	}

	log.Println("Server exited")
}
