package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/kyangconn/music-online-web/internal/config"
	"github.com/kyangconn/music-online-web/internal/handler"
	"github.com/kyangconn/music-online-web/internal/middleware"
	"github.com/kyangconn/music-online-web/internal/pkg/database"
	"github.com/kyangconn/music-online-web/internal/repository"
	"github.com/kyangconn/music-online-web/internal/service"
)

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

			// 管理员路由
			admin := protected.Group("")
			admin.Use(middleware.RoleMiddleware("admin"))
			{
				admin.GET("", userHandler.ListUsers)
				admin.GET("/:id", userHandler.GetUserByID)
			}
		}
	}

	// 10. 启动服务器
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
