package database

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"gorm.io/gorm"

	"github.com/kyangconn/music-online-go/internal/domain"
	pklog "github.com/kyangconn/music-online-go/internal/pkg/log"
)

type schemaMigration struct {
	Version   int64 `gorm:"primaryKey;autoIncrement:false"`
	Name      string
	AppliedAt time.Time
}

func (*schemaMigration) TableName() string {
	return "schema_migrations"
}

type migration struct {
	version int64
	name    string
	up      func(*gorm.DB) error
}

// Keep this list append-only. Applied versions must never be reordered,
// renamed, or reused; every schema change gets a new, higher version.
var migrations = []migration{
	{
		version: 1,
		name:    "initial_schema",
		up: func(db *gorm.DB) error {
			return db.AutoMigrate(
				&domain.User{},
				&domain.Music{},
				&domain.UserMusicLike{},
				&domain.MusicTag{},
			)
		},
	},
	{
		version: 2,
		name:    "backfill_music_file_hashes",
		up:      backfillMusicFileHashes,
	},
}

// Migrate applies every pending schema migration in version order. Each
// migration and its history record share one transaction, so a failed upgrade
// is safe to retry after the underlying problem is fixed.
func Migrate() error {
	if DB == nil {
		return errors.New("database is not connected")
	}
	return migrate(DB, migrations)
}

func migrate(db *gorm.DB, available []migration) error {
	if db == nil {
		return errors.New("database is nil")
	}
	if err := validateMigrations(available); err != nil {
		return err
	}
	if err := ensureMigrationTable(db); err != nil {
		return fmt.Errorf("create migration history table: %w", err)
	}

	var applied []schemaMigration
	if err := db.Order("version ASC").Find(&applied).Error; err != nil {
		return fmt.Errorf("load migration history: %w", err)
	}

	known := make(map[int64]migration, len(available))
	for _, item := range available {
		known[item.version] = item
	}
	appliedVersions := make(map[int64]struct{}, len(applied))
	for _, record := range applied {
		item, ok := known[record.Version]
		if !ok {
			return fmt.Errorf("database has unknown migration version %d; restore a compatible backup or use a newer binary", record.Version)
		}
		if record.Name != item.name {
			return fmt.Errorf("migration %d name mismatch: database has %q, binary expects %q", record.Version, record.Name, item.name)
		}
		appliedVersions[record.Version] = struct{}{}
	}

	for _, item := range available {
		if _, ok := appliedVersions[item.version]; ok {
			continue
		}

		if err := db.Transaction(func(tx *gorm.DB) error {
			if err := item.up(tx); err != nil {
				return err
			}
			return tx.Create(&schemaMigration{
				Version:   item.version,
				Name:      item.name,
				AppliedAt: time.Now().UTC(),
			}).Error
		}); err != nil {
			return fmt.Errorf("apply migration %d (%s): %w", item.version, item.name, err)
		}
		pklog.Infof("Applied database migration %d (%s)", item.version, item.name)
	}

	return nil
}

func validateMigrations(available []migration) error {
	var previousVersion int64
	for index, item := range available {
		if item.version <= 0 {
			return fmt.Errorf("migration at index %d has invalid version %d", index, item.version)
		}
		if item.version <= previousVersion {
			return fmt.Errorf("migration versions must be strictly increasing: %d follows %d", item.version, previousVersion)
		}
		if item.name == "" {
			return fmt.Errorf("migration %d has an empty name", item.version)
		}
		if item.up == nil {
			return fmt.Errorf("migration %d (%s) has no upgrade function", item.version, item.name)
		}
		previousVersion = item.version
	}
	return nil
}

func ensureMigrationTable(db *gorm.DB) error {
	return db.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (
		version BIGINT PRIMARY KEY,
		name VARCHAR(255) NOT NULL,
		applied_at TIMESTAMP NOT NULL
	)`).Error
}

func backfillMusicFileHashes(db *gorm.DB) error {
	var musics []domain.Music
	if err := db.Where("file_hash = ? AND path <> ?", "", "").Find(&musics).Error; err != nil {
		return fmt.Errorf("failed to find music file hashes to backfill: %w", err)
	}

	for _, music := range musics {
		fileHash, err := hashFile(music.Path)
		if err != nil {
			pklog.Warnf("Skipping file hash backfill for music %d (%s): %v", music.ID, music.Path, err)
			continue
		}
		if err := db.Model(&domain.Music{}).
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
	file, err := os.Open(filepath.Clean(path))
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
