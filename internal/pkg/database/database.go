package database

import (
	"fmt"
	"log"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/kyangconn/music-online-go/internal/config"
	"github.com/kyangconn/music-online-go/internal/domain"
)

var DB *gorm.DB

func Connect() error {
	cfg := config.AppConfig.Database

	dialector, err := getDialector(cfg)
	if err != nil {
		return err
	}

	DB, err = gorm.Open(dialector, &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
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

	log.Println("Database connected successfully")
	return nil
}

func getDialector(cfg config.DatabaseConfig) (gorm.Dialector, error) {
	switch cfg.Type {
	case "sqlite":
		path := cfg.Path
		if path == "" {
			path = "music.db"
		}
		return sqlite.Open(path), nil
	case "postgres", "":
		dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%s sslmode=%s",
			cfg.Host, cfg.User, cfg.Password, cfg.Name, cfg.Port, cfg.SSLMode)
		return postgres.Open(dsn), nil
	default:
		return nil, fmt.Errorf("unsupported database type: %s", cfg.Type)
	}
}

func AutoMigrate() error {
	return DB.AutoMigrate(
		&domain.User{},
		&domain.Music{},
		&domain.UserMusicLike{},
		&domain.MusicTag{},
	)
}

func Close() error {
	sqlDB, err := DB.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}
