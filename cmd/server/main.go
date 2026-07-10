// main.go - 音乐在线平台服务器主程序
// 负责初始化配置、数据库、依赖注入，然后启动HTTP服务器
package main

import (
	"context"
	"embed"
	"errors"
	"flag"
	"fmt"
	"io/fs"
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
	pklog "github.com/kyangconn/music-online-go/internal/pkg/log"
	"github.com/kyangconn/music-online-go/internal/repository"
	"github.com/kyangconn/music-online-go/internal/router"
	"github.com/kyangconn/music-online-go/internal/service"
	"github.com/kyangconn/music-online-go/internal/version"
)

//go:embed dist/*
var webDist embed.FS

func main() {
	parseFlags()
	pklog.Infof("%s", version.String())

	if err := config.LoadConfig(); err != nil {
		pklog.Fatalf("Failed to load config: %v", err)
	}
	pklog.Init(config.AppConfig.Server.LogFile)

	if err := database.Connect(); err != nil {
		pklog.Fatalf("Failed to connect database: %v", err)
	}
	if err := database.AutoMigrate(); err != nil {
		pklog.Fatalf("Failed to migrate database: %v", err)
	}
	if err := bootstrapAdmin(context.Background()); err != nil {
		pklog.Fatalf("Failed to bootstrap admin user: %v", err)
	}
	defer func() {
		if err := database.Close(); err != nil {
			pklog.Errorf("Failed to close database: %v", err)
		}
	}()

	handlers := initDependencies()
	r := router.New(handlers, database.DB)
	configureStaticAssets(r)
	startServer(r)
}

func parseFlags() {
	configFile := flag.String("config-file", "", "Path to config YAML file")
	logFile := flag.String("log-file", "", "Path to log file (overrides config/env)")
	flag.Parse()

	setEnvIfNotEmpty("MO_CONFIG_FILE", *configFile)
	setEnvIfNotEmpty("MO_LOG_FILE", *logFile)
}

func setEnvIfNotEmpty(key, val string) {
	if val == "" {
		return
	}
	if err := os.Setenv(key, val); err != nil {
		pklog.Fatalf("Failed to set %s: %v", key, err)
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

func bootstrapAdmin(ctx context.Context) error {
	cfg := config.AppConfig.AdminBootstrap
	if !cfg.Enabled {
		return nil
	}

	userRepo := repository.NewUserRepository(database.DB)
	userService := service.NewUserService(userRepo)
	user, created, err := userService.BootstrapAdmin(ctx, service.BootstrapAdminRequest{
		Username:      cfg.Username,
		Email:         cfg.Email,
		Password:      cfg.Password,
		FullName:      cfg.FullName,
		ResetPassword: cfg.ResetPassword,
	})
	if err != nil {
		return err
	}
	if created {
		pklog.Infof("Bootstrap admin user created: %s", user.Username)
	} else {
		pklog.Infof("Bootstrap admin user ensured: %s", user.Username)
	}
	return nil
}

func configureStaticAssets(r *gin.Engine) {
	distFS, err := fs.Sub(webDist, "dist")
	if err != nil {
		pklog.Fatalf("Failed to create dist fs: %v", err)
	}

	r.NoRoute(func(c *gin.Context) {
		if strings.HasPrefix(c.Request.URL.Path, "/api") {
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
			return
		}
		path := strings.TrimPrefix(c.Request.URL.Path, "/")
		if path == "" {
			path = "index.html"
		}

		data, contentType, err := readEmbedFile(distFS, path)
		if err != nil {
			data, contentType, err = readEmbedFile(distFS, "index.html")
			if err != nil {
				c.String(http.StatusNotFound, "Not Found")
				return
			}
		}

		if contentType != "" {
			c.Data(http.StatusOK, contentType, data)
		} else {
			c.Data(http.StatusOK, "application/octet-stream", data)
		}
	})
}

func readEmbedFile(fsys fs.FS, name string) ([]byte, string, error) {
	f, err := fsys.Open(name)
	if err != nil {
		return nil, "", err
	}
	defer func() {
		err := f.Close()
		if err != nil {
			pklog.Errorf("Failed to close embedded file %s: %v", name, err)
		}
	}()

	stat, err := f.Stat()
	if err != nil {
		return nil, "", err
	}

	if stat.IsDir() {
		return nil, "", fmt.Errorf("is directory")
	}

	data, err := fs.ReadFile(fsys, name)
	if err != nil {
		return nil, "", err
	}

	contentType := mimeTypeByExt(name)
	return data, contentType, nil
}

func mimeTypeByExt(name string) string {
	switch {
	case strings.HasSuffix(name, ".html"):
		return "text/html; charset=utf-8"
	case strings.HasSuffix(name, ".css"):
		return "text/css; charset=utf-8"
	case strings.HasSuffix(name, ".js"):
		return "application/javascript; charset=utf-8"
	case strings.HasSuffix(name, ".json"):
		return "application/json; charset=utf-8"
	case strings.HasSuffix(name, ".png"):
		return "image/png"
	case strings.HasSuffix(name, ".jpg"), strings.HasSuffix(name, ".jpeg"):
		return "image/jpeg"
	case strings.HasSuffix(name, ".svg"):
		return "image/svg+xml"
	case strings.HasSuffix(name, ".ico"):
		return "image/x-icon"
	case strings.HasSuffix(name, ".woff"):
		return "font/woff"
	case strings.HasSuffix(name, ".woff2"):
		return "font/woff2"
	default:
		return "application/octet-stream"
	}
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
		pklog.Infof("Server starting on http://localhost:%s", port)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			if strings.Contains(err.Error(), "address already in use") {
				pklog.Fatalf("Port %s is already in use. Close the other program or use a different port.", port)
			}
			pklog.Fatalf("Failed to start server: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	pklog.Infof("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		pklog.Fatalf("Server forced to shutdown: %v", err)
	}
	pklog.Infof("Server exited")
}
