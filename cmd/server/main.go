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

	"github.com/kyangconn/music-online-web/internal/config"
	"github.com/kyangconn/music-online-web/internal/handler"
	"github.com/kyangconn/music-online-web/internal/middleware"
	"github.com/kyangconn/music-online-web/internal/pkg/database"
	"github.com/kyangconn/music-online-web/internal/repository"
	"github.com/kyangconn/music-online-web/internal/service"
)

// 嵌入前端构建产物
//
//go:embed dist/*
var webDist embed.FS

func main() {
	// 1. 加载配置
	if err := config.LoadConfig(); err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// 2. 连接数据库
	if err := database.Connect(); err != nil {
		log.Fatalf("Failed to connect database: %v", err)
	}
	defer database.Close()

	// 3. 自动迁移表结构
	if err := database.AutoMigrate(); err != nil {
		log.Fatalf("Failed to migrate database: %v", err)
	}

	// 4. 初始化依赖
	userRepo := repository.NewUserRepository(database.DB)
	userService := service.NewUserService(userRepo)
	userHandler := handler.NewUserHandler(userService)

	musicRepo := repository.NewMusicRepository(database.DB)
	musicService := service.NewMusicService(musicRepo)
	musicHandler := handler.NewMusicHandler(musicService)

	adminHandler := handler.NewAdminHandler(userService, musicService)

	// 5. 设置Gin模式
	if config.AppConfig.Server.Mode == "release" {
		gin.SetMode(gin.ReleaseMode)
	}

	// 6. 创建Gin引擎
	router := gin.New()

	// 7. 注册中间件
	router.Use(
		middleware.LoggerMiddleware(),
		middleware.CORSMiddleware(),
		middleware.RateLimitMiddleware(rate.Limit(20), 50), // 全局限流：每秒20请求，突发50
		gin.Recovery(),
	)

	// 8. 注册健康检查路由
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status": "ok",
			"time":   time.Now().Format(time.RFC3339),
		})
	})

	// 9. 注册API路由
	api := router.Group("/api/v1")
	{
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

			// 管理员路由 (使用 StrictAdminMiddleware 强制查库验证)
			admin := protected.Group("/admin")
			admin.Use(middleware.StrictAdminMiddleware(database.DB))
			{
				admin.GET("/users", adminHandler.ListUsers)
				admin.PUT("/users/:id/status", adminHandler.UpdateUserStatus)
				admin.PUT("/users/:id/role", adminHandler.UpdateUserRole)
				admin.DELETE("/musics/:id", adminHandler.DeleteMusic)
			}
		}

		// 音乐模块路由
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

	// 10. 静态资源托管 (支持 SPA)
	// 获取 dist 子目录文件系统
	distFS, err := fs.Sub(webDist, "dist")
	if err != nil {
		log.Fatalf("Failed to create dist fs: %v", err)
	}
	httpFS := http.FS(distFS)

	router.NoRoute(func(c *gin.Context) {
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
	})

	// 11. 启动服务器
	port := config.AppConfig.Server.Port
	if port == "" {
		port = "8080"
	}

	srv := &http.Server{
		Addr:         ":" + port,
		Handler:      router,
		ReadTimeout:  time.Duration(config.AppConfig.Server.ReadTimeout) * time.Second,
		WriteTimeout: time.Duration(config.AppConfig.Server.WriteTimeout) * time.Second,
	}

	// 优雅关机
	go func() {
		log.Printf("Server starting on port %s", port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

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
