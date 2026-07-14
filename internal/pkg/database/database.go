// Package database database.go - 数据库连接
// 提供数据库连接、版本化迁移、关闭等基础操作，支持 PostgreSQL 和 SQLite
package database

import (
	"fmt"
	"os"
	"path/filepath"
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

	logLevel := logger.Info
	if config.AppConfig.Server.Mode == "release" {
		logLevel = logger.Warn
	}

	DB, err = gorm.Open(dialector, &gorm.Config{
		Logger: logger.Default.LogMode(logLevel),
	})
	if err != nil {
		return fmt.Errorf("failed to connect database: %w", err)
	}

	sqlDB, err := DB.DB()
	if err != nil {
		return err
	}

	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetMaxOpenConns(100)
	sqlDB.SetConnMaxLifetime(time.Hour)

	pklog.Infof("Database connected successfully")
	return nil
}

func getDialector(cfg config.DatabaseConfig) (gorm.Dialector, error) {
	switch cfg.Type {
	case "sqlite", "":
		path, err := prepareSQLitePath(cfg.Path)
		if err != nil {
			return nil, err
		}
		return sqlite.Open(path), nil
	case "postgres":
		if !hasPostgresConfig(cfg) {
			pklog.Infof("PostgreSQL config is incomplete; falling back to SQLite database")
			path, err := prepareSQLitePath(cfg.Path)
			if err != nil {
				return nil, err
			}
			return sqlite.Open(path), nil
		}
		dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%s sslmode=%s",
			cfg.Host, cfg.User, cfg.Password, cfg.Name, cfg.Port, cfg.SSLMode)
		return postgres.Open(dsn), nil
	default:
		return nil, fmt.Errorf("unsupported database type: %s", cfg.Type)
	}
}

func hasPostgresConfig(cfg config.DatabaseConfig) bool {
	return cfg.Host != "" && cfg.User != "" && cfg.Name != ""
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
