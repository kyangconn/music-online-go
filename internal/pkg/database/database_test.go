package database

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"github.com/kyangconn/music-online-go/internal/domain"
)

func TestBackfillMusicFileHashes(t *testing.T) {
	originalDB := DB
	t.Cleanup(func() { DB = originalDB })

	var err error
	DB, err = gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open in-memory database: %v", err)
	}
	if err := DB.AutoMigrate(&domain.User{}, &domain.Music{}); err != nil {
		t.Fatalf("migrate test database: %v", err)
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

	if err := backfillMusicFileHashes(); err != nil {
		t.Fatalf("backfill file hashes: %v", err)
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
}
