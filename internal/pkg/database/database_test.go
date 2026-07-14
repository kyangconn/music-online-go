package database

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"github.com/kyangconn/music-online-go/internal/domain"
)

func openTestDatabase(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open in-memory database: %v", err)
	}
	return db
}

func TestMigrateFreshDatabaseAndRemainIdempotent(t *testing.T) {
	db := openTestDatabase(t)

	if err := migrate(db, migrations); err != nil {
		t.Fatalf("migrate fresh database: %v", err)
	}
	for _, table := range []string{"users", "vinyl", "user_music_likes", "music_tags", "schema_migrations"} {
		if !db.Migrator().HasTable(table) {
			t.Fatalf("expected table %q to exist", table)
		}
	}

	var firstRun []schemaMigration
	if err := db.Order("version ASC").Find(&firstRun).Error; err != nil {
		t.Fatalf("load first migration history: %v", err)
	}
	if len(firstRun) != len(migrations) {
		t.Fatalf("expected %d migration records, got %d", len(migrations), len(firstRun))
	}

	if err := migrate(db, migrations); err != nil {
		t.Fatalf("repeat migrations: %v", err)
	}
	var secondRun []schemaMigration
	if err := db.Order("version ASC").Find(&secondRun).Error; err != nil {
		t.Fatalf("load repeated migration history: %v", err)
	}
	if len(secondRun) != len(firstRun) {
		t.Fatalf("repeat migration changed history size: got %d, want %d", len(secondRun), len(firstRun))
	}
	for index := range firstRun {
		if secondRun[index] != firstRun[index] {
			t.Fatalf("repeat migration changed record %d: got %#v, want %#v", index, secondRun[index], firstRun[index])
		}
	}
}

func TestMigrateRollsBackFailedMigration(t *testing.T) {
	db := openTestDatabase(t)
	boom := errors.New("boom")
	failing := []migration{{
		version: 1,
		name:    "failing_migration",
		up: func(tx *gorm.DB) error {
			if err := tx.Exec("CREATE TABLE should_rollback (id INTEGER PRIMARY KEY)").Error; err != nil {
				return err
			}
			return boom
		},
	}}

	err := migrate(db, failing)
	if !errors.Is(err, boom) {
		t.Fatalf("expected migration failure %v, got %v", boom, err)
	}
	if db.Migrator().HasTable("should_rollback") {
		t.Fatal("failed migration left its schema changes behind")
	}
	var count int64
	if err := db.Model(&schemaMigration{}).Count(&count).Error; err != nil {
		t.Fatalf("count migration history: %v", err)
	}
	if count != 0 {
		t.Fatalf("failed migration was recorded as applied: count=%d", count)
	}
}

func TestMigrateRejectsUnknownDatabaseVersion(t *testing.T) {
	db := openTestDatabase(t)
	if err := ensureMigrationTable(db); err != nil {
		t.Fatalf("create migration table: %v", err)
	}
	if err := db.Create(&schemaMigration{
		Version:   999,
		Name:      "future_schema",
		AppliedAt: time.Now().UTC(),
	}).Error; err != nil {
		t.Fatalf("record future migration: %v", err)
	}

	err := migrate(db, migrations)
	if err == nil || !strings.Contains(err.Error(), "unknown migration version 999") {
		t.Fatalf("expected unknown-version error, got %v", err)
	}
}

func TestMigrateBackfillsExistingMusicFileHashes(t *testing.T) {
	originalDB := DB
	t.Cleanup(func() { DB = originalDB })

	DB = openTestDatabase(t)
	if err := DB.AutoMigrate(&domain.User{}, &domain.Music{}); err != nil {
		t.Fatalf("create legacy schema: %v", err)
	}

	content := []byte("existing music content")
	path := filepath.Join(t.TempDir(), "music.mp3")
	if err := os.WriteFile(path, content, 0600); err != nil {
		t.Fatalf("write test music: %v", err)
	}

	user := domain.User{Username: "hash-owner", Password: "unused"}
	if err := DB.Create(&user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
	musics := []domain.Music{
		{Title: "Existing", Artist: "Artist", Path: path, UserID: user.ID},
		{Title: "Missing", Artist: "Artist", Path: filepath.Join(t.TempDir(), "missing.mp3"), UserID: user.ID},
	}
	if err := DB.Create(&musics).Error; err != nil {
		t.Fatalf("create music records: %v", err)
	}

	if err := Migrate(); err != nil {
		t.Fatalf("upgrade existing database: %v", err)
	}

	var existing domain.Music
	if err := DB.First(&existing, musics[0].ID).Error; err != nil {
		t.Fatalf("reload existing music: %v", err)
	}
	expectedHash := sha256.Sum256(content)
	if existing.FileHash != hex.EncodeToString(expectedHash[:]) {
		t.Fatalf("unexpected file hash: got %q", existing.FileHash)
	}

	var missing domain.Music
	if err := DB.First(&missing, musics[1].ID).Error; err != nil {
		t.Fatalf("reload missing music: %v", err)
	}
	if missing.FileHash != "" {
		t.Fatalf("missing file should keep an empty hash, got %q", missing.FileHash)
	}

	var history []schemaMigration
	if err := DB.Order("version ASC").Find(&history).Error; err != nil {
		t.Fatalf("load migration history: %v", err)
	}
	if len(history) != len(migrations) {
		t.Fatalf("expected %d applied migrations, got %d", len(migrations), len(history))
	}
}
