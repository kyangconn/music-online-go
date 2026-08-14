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
	"net"
	"net/http"
	"os"
	"os/signal"
	"regexp"
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

var fingerprintedAssetPattern = regexp.MustCompile(`(?:^|/)[^/]+-[A-Za-z0-9_-]{8,}\.[^/]+$`)

const (
	cacheControlImmutable  = "public, max-age=31536000, immutable"
	cacheControlRevalidate = "no-cache"
	cacheControlShort      = "public, max-age=3600"
)

func main() {
	parseFlags()

	if err := config.LoadConfig(); err != nil {
		pklog.Fatalf("Failed to load config: %v", err)
	}
	pklog.Init(config.AppConfig.Server.LogFile, pklog.Options{
		Level:      config.AppConfig.Logging.Level,
		MaxSizeMB:  config.AppConfig.Logging.MaxSizeMB,
		MaxBackups: config.AppConfig.Logging.MaxBackups,
		MaxAgeDays: config.AppConfig.Logging.MaxAgeDays,
		Compress:   config.AppConfig.Logging.Compress,
		LocalTime:  config.AppConfig.Logging.LocalTime,
	})
	pklog.Infof("%s", version.String())
	if config.AppConfig.SourceFile == "" {
		pklog.Infof("No config file found; using defaults and environment overrides")
	} else {
		pklog.Infof("Loaded config file: %s", config.AppConfig.SourceFile)
	}

	if err := database.Connect(); err != nil {
		pklog.Fatalf("Failed to connect database: %v", err)
	}
	if err := database.Migrate(); err != nil {
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

	handlers, mediaLibraryService, analysisService, err := initDependencies()
	if err != nil {
		pklog.Fatalf("Failed to initialize application dependencies: %v", err)
	}
	r, err := router.New(handlers, database.DB, *config.AppConfig)
	if err != nil {
		pklog.Fatalf("Failed to configure router: %v", err)
	}
	configureStaticAssets(r)
	workerContext, stopWorkers := context.WithCancel(context.Background())
	if err := mediaLibraryService.Start(workerContext); err != nil {
		pklog.Fatalf("Failed to start media library scanner: %v", err)
	}
	if err := analysisService.Start(workerContext); err != nil {
		stopWorkers()
		pklog.Fatalf("Failed to start music analysis worker: %v", err)
	}
	startServer(r)
	stopWorkers()
	shutdownContext, cancelShutdown := context.WithTimeout(
		context.Background(),
		time.Duration(config.AppConfig.Server.ShutdownTimeout)*time.Second,
	)
	defer cancelShutdown()
	if err := mediaLibraryService.Shutdown(shutdownContext); err != nil {
		pklog.Errorf("Media library scanner did not stop cleanly: %v", err)
	}
	if err := analysisService.Shutdown(shutdownContext); err != nil {
		pklog.Errorf("Music analysis worker did not stop cleanly: %v", err)
	}
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

func initDependencies() (*router.Handlers, service.MediaLibraryService, service.MusicAnalysisService, error) {
	userRepo := repository.NewUserRepository(database.DB)
	sessionRepo := repository.NewSessionRepository(database.DB)
	userService := service.NewUserService(userRepo, sessionRepo, config.AppConfig.Server.UploadDir, config.AppConfig.JWT)

	classificationConfig := config.AppConfig.Classification
	presetPolicy := classificationConfig.PresetRulePolicy()
	musicRepo := repository.NewMusicRepository(database.DB, presetPolicy)
	browseRepo := repository.NewBrowseRepository(database.DB)
	playlistRepo := repository.NewPlaylistRepository(database.DB)
	presetRepo := repository.NewPresetRepository(database.DB, presetPolicy)
	analysisRepo := repository.NewMusicAnalysisRepository(database.DB)
	mediaLibraryRepo := repository.NewMediaLibraryRepository(database.DB, presetPolicy)
	subsystem := service.NewMusicSubsystem(service.MusicSubsystemRepositories{
		Music: musicRepo, Preset: presetRepo, Analysis: analysisRepo, MediaLibrary: mediaLibraryRepo,
	}, *config.AppConfig)
	browseService := service.NewBrowseService(browseRepo, *config.AppConfig)
	playlistService := service.NewPlaylistService(playlistRepo, subsystem.Music)
	presetService := service.NewPresetClassificationService(presetRepo, classificationConfig.Enabled)
	sqlDB, err := database.DB.DB()
	if err != nil {
		return nil, nil, nil, fmt.Errorf("get database statistics handle: %w", err)
	}

	return &router.Handlers{
		User:     handler.NewUserHandler(userService, config.AppConfig.JWT),
		Music:    handler.NewMusicHandler(subsystem.Music, config.AppConfig.Access),
		Browse:   handler.NewBrowseHandler(browseService),
		Playlist: handler.NewPlaylistHandler(playlistService),
		Preset:   handler.NewPresetClassificationHandler(presetService),
		Analysis: handler.NewMusicAnalysisHandler(subsystem.Analysis),
		MusicTag: handler.NewMusicTagHandler(subsystem.Music),
		Admin:    handler.NewAdminHandler(userService, subsystem.Music, subsystem.MediaLibrary, *config.AppConfig, sqlDB),
	}, subsystem.MediaLibrary, subsystem.Analysis, nil
}

func bootstrapAdmin(ctx context.Context) error {
	cfg := config.AppConfig.AdminBootstrap
	if !cfg.Enabled {
		return nil
	}

	userRepo := repository.NewUserRepository(database.DB)
	sessionRepo := repository.NewSessionRepository(database.DB)
	userService := service.NewUserService(userRepo, sessionRepo, config.AppConfig.Server.UploadDir, config.AppConfig.JWT)
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
		requestedPath := strings.TrimPrefix(c.Request.URL.Path, "/")
		if requestedPath == "" {
			requestedPath = "index.html"
		}

		servedPath := requestedPath
		data, contentType, err := readEmbedFile(distFS, servedPath)
		if err != nil {
			servedPath = "index.html"
			data, contentType, err = readEmbedFile(distFS, servedPath)
			if err != nil {
				c.String(http.StatusNotFound, "Not Found")
				return
			}
		}

		c.Header("Cache-Control", cacheControlForAsset(servedPath))
		if contentType != "" {
			c.Data(http.StatusOK, contentType, data)
		} else {
			c.Data(http.StatusOK, "application/octet-stream", data)
		}
	})
}

func cacheControlForAsset(name string) string {
	cleanName := strings.TrimPrefix(name, "/")
	switch cleanName {
	case "", "index.html", "sw.js", "manifest.json", "manifest.webmanifest":
		return cacheControlRevalidate
	}

	if strings.HasPrefix(cleanName, "assets/") && fingerprintedAssetPattern.MatchString(cleanName) {
		return cacheControlImmutable
	}

	return cacheControlShort
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
		Addr:              serverAddress(config.AppConfig.Server.ListenAddress, port),
		Handler:           r,
		ReadHeaderTimeout: time.Duration(config.AppConfig.Server.ReadHeaderTimeout) * time.Second,
		ReadTimeout:       time.Duration(config.AppConfig.Server.ReadTimeout) * time.Second,
		WriteTimeout:      time.Duration(config.AppConfig.Server.WriteTimeout) * time.Second,
		IdleTimeout:       time.Duration(config.AppConfig.Server.IdleTimeout) * time.Second,
	}

	go func() {
		pklog.Infof("Server listening on %s", srv.Addr)
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

	ctx, cancel := context.WithTimeout(
		context.Background(),
		time.Duration(config.AppConfig.Server.ShutdownTimeout)*time.Second,
	)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		pklog.Fatalf("Server forced to shutdown: %v", err)
	}
	pklog.Infof("Server exited")
}

func serverAddress(listenAddress, port string) string {
	if strings.TrimSpace(listenAddress) == "" {
		return ":" + port
	}
	return net.JoinHostPort(strings.TrimSpace(listenAddress), port)
}
