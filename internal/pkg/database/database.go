// Package database database.go - 数据库连接
// 提供数据库连接、自动迁移、关闭等基础操作，支持 PostgreSQL 和 SQLite
package database

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/kyangconn/music-online-go/internal/config"
	"github.com/kyangconn/music-online-go/internal/domain"
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

func AutoMigrate() error {
	if err := DB.AutoMigrate(
		&domain.User{},
		&domain.Music{},
		&domain.UserMusicLike{},
		&domain.MusicTag{},
	); err != nil {
		return err
	}
	return backfillMusicFileHashes()
}

func backfillMusicFileHashes() error {
	var musics []domain.Music
	if err := DB.Where("file_hash = ? AND path <> ?", "", "").Find(&musics).Error; err != nil {
		return fmt.Errorf("failed to find music file hashes to backfill: %w", err)
	}

	for _, music := range musics {
		fileHash, err := hashFile(music.Path)
		if err != nil {
			pklog.Warnf("Skipping file hash backfill for music %d (%s): %v", music.ID, music.Path, err)
			continue
		}
		if err := DB.Model(&domain.Music{}).
			Where("id = ? AND file_hash = ?", music.ID, "").
			Update("file_hash", fileHash).Error; err != nil {
			return fmt.Errorf("failed to backfill file hash for music %d: %w", music.ID, err)
		}
	}

	if len(musics) > 0 {
		pklog.Infof("Checked %d existing music files for hash backfill", len(musics))
	}
	return nil
}

func hashFile(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer func() {
		if cerr := file.Close(); cerr != nil && !errors.Is(cerr, io.EOF) && !errors.Is(cerr, io.ErrUnexpectedEOF) {
			pklog.Warnf("Failed to close music file %s after hashing: %v", path, cerr)
		}
	}()

	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(hasher.Sum(nil)), nil
}

func Close() error {
	sqlDB, err := DB.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}
