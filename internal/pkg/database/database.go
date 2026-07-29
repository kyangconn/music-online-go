// Package database database.go - 数据库连接
// 提供数据库连接、版本化迁移、关闭等基础操作，支持 PostgreSQL 和 SQLite
package database

import (
	"fmt"
	stdlog "log"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/kyangconn/music-online-go/internal/config"
	pklog "github.com/kyangconn/music-online-go/internal/pkg/log"
)

var DB *gorm.DB

func Connect() error {
	cfg := config.AppConfig.Database

	dialector, err := getDialector(cfg)
	if err != nil {
		return err
	}

	DB, err = gorm.Open(dialector, &gorm.Config{
		Logger: newGORMLogger(gormLogLevel(cfg.LogLevel, config.AppConfig.Server.Mode)),
	})
	if err != nil {
		return fmt.Errorf("failed to connect database: %w", err)
	}

	sqlDB, err := DB.DB()
	if err != nil {
		return fmt.Errorf("get database connection pool: %w", err)
	}

	maxOpen, maxIdle := connectionPoolLimits(cfg)
	sqlDB.SetMaxOpenConns(maxOpen)
	sqlDB.SetMaxIdleConns(maxIdle)
	sqlDB.SetConnMaxLifetime(time.Duration(cfg.ConnectionMaxLifetimeMinutes) * time.Minute)
	sqlDB.SetConnMaxIdleTime(time.Duration(cfg.ConnectionMaxIdleTimeMinutes) * time.Minute)

	pklog.Infof("Database connected successfully")
	return nil
}

func newGORMLogger(level logger.LogLevel) logger.Interface {
	return logger.New(
		stdlog.New(os.Stdout, "\r\n", stdlog.LstdFlags),
		gormLoggerConfig(level),
	)
}

func gormLoggerConfig(level logger.LogLevel) logger.Config {
	return logger.Config{
		SlowThreshold:             200 * time.Millisecond,
		LogLevel:                  level,
		IgnoreRecordNotFoundError: true,
		ParameterizedQueries:      true,
		Colorful:                  false,
	}
}

func gormLogLevel(configured, serverMode string) logger.LogLevel {
	switch configured {
	case "silent":
		return logger.Silent
	case "error":
		return logger.Error
	case "warn":
		return logger.Warn
	case "info":
		return logger.Info
	default:
		if serverMode == "release" {
			return logger.Warn
		}
		return logger.Info
	}
}

func getDialector(cfg config.DatabaseConfig) (gorm.Dialector, error) {
	if err := config.ValidateDatabaseConfig(cfg); err != nil {
		return nil, err
	}

	switch cfg.Type {
	case "sqlite":
		path, err := prepareSQLitePath(cfg.Path)
		if err != nil {
			return nil, err
		}
		return sqlite.Open(path), nil
	case "postgres":
		return postgres.Open(postgresDSN(cfg)), nil
	default:
		return nil, fmt.Errorf("unsupported database type: %s", cfg.Type)
	}
}

func postgresDSN(cfg config.DatabaseConfig) string {
	credentials := url.User(cfg.User)
	if cfg.Password != "" {
		credentials = url.UserPassword(cfg.User, cfg.Password)
	}

	query := url.Values{}
	query.Set("connect_timeout", strconv.Itoa(cfg.ConnectTimeoutSeconds))
	query.Set("sslmode", cfg.SSLMode)

	return (&url.URL{
		Scheme:   "postgres",
		User:     credentials,
		Host:     net.JoinHostPort(cfg.Host, cfg.Port),
		Path:     cfg.Name,
		RawQuery: query.Encode(),
	}).String()
}

func connectionPoolLimits(cfg config.DatabaseConfig) (int, int) {
	maxOpen := cfg.MaxOpenConnections
	maxIdle := cfg.MaxIdleConnections
	if cfg.Type == "sqlite" {
		if maxOpen == 0 {
			maxOpen = 1
		}
		if maxIdle == 0 {
			maxIdle = 1
		}
	} else {
		if maxOpen == 0 {
			maxOpen = 25
		}
		if maxIdle == 0 {
			maxIdle = 5
		}
	}
	if maxIdle > maxOpen {
		maxIdle = maxOpen
	}
	return maxOpen, maxIdle
}

func prepareSQLitePath(path string) (string, error) {
	if path == "" {
		path = "music.db"
	}
	if path == ":memory:" || strings.HasPrefix(path, "file:") {
		return path, nil
	}

	dir := filepath.Dir(path)
	if dir == "." || dir == "" {
		return path, nil
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return "", fmt.Errorf("failed to create sqlite directory: %w", err)
	}
	return path, nil
}

func Close() error {
	sqlDB, err := DB.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}
