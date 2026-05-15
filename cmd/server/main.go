// main.go - 音乐在线平台服务器主程序
// 负责初始化配置、数据库、依赖注入，然后启动HTTP服务器
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
	"github.com/kyangconn/music-online-go/internal/config"
	"github.com/kyangconn/music-online-go/internal/handler"
	"github.com/kyangconn/music-online-go/internal/pkg/database"
	"github.com/kyangconn/music-online-go/internal/repository"
	"github.com/kyangconn/music-online-go/internal/router"
	"github.com/kyangconn/music-online-go/internal/service"
)

//go:embed dist/*
var webDist embed.FS

func main() {
	initConfigAndDatabase()
	handlers := initDependencies()
	r := router.New(handlers, database.DB)
	configureStaticAssets(r)
	startServer(r)
}

func initConfigAndDatabase() {
	if err := config.LoadConfig(); err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}
	if err := database.Connect(); err != nil {
		log.Fatalf("Failed to connect database: %v", err)
	}
	defer database.Close()
	if err := database.AutoMigrate(); err != nil {
		log.Fatalf("Failed to migrate database: %v", err)
	}
}

func initDependencies() *router.Handlers {
	userRepo := repository.NewUserRepository(database.DB)
	userService := service.NewUserService(userRepo)

	musicRepo := repository.NewMusicRepository(database.DB)
	musicService := service.NewMusicService(musicRepo)

	musicTagRepo := repository.NewMusicTagRepository(database.DB)
	musicTagService := service.NewMusicTagService(musicTagRepo)

	return &router.Handlers{
		User:     handler.NewUserHandler(userService),
		Music:    handler.NewMusicHandler(musicService),
		MusicTag: handler.NewMusicTagHandler(musicTagService),
		Admin:    handler.NewAdminHandler(userService, musicService, musicTagService),
	}
}

func configureStaticAssets(r *gin.Engine) {
	distFS, err := fs.Sub(webDist, "dist")
	if err != nil {
		log.Fatalf("Failed to create dist fs: %v", err)
	}
	httpFS := http.FS(distFS)

	r.NoRoute(func(c *gin.Context) {
		if strings.HasPrefix(c.Request.URL.Path, "/api") {
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
			return
		}
		path := strings.TrimPrefix(c.Request.URL.Path, "/")
		if path == "" {
			path = "index.html"
		}
		if _, err := distFS.Open(path); err != nil {
			c.FileFromFS("index.html", httpFS)
			return
		}
		c.FileFromFS(path, httpFS)
	})
}

func startServer(r *gin.Engine) {
	port := config.AppConfig.Server.Port
	if port == "" {
		port = "8080"
	}

	srv := &http.Server{
		Addr:         ":" + port,
		Handler:      r,
		ReadTimeout:  time.Duration(config.AppConfig.Server.ReadTimeout) * time.Second,
		WriteTimeout: time.Duration(config.AppConfig.Server.WriteTimeout) * time.Second,
	}

	go func() {
		log.Printf("Server starting on port %s", port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

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
