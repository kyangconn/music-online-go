package database

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/kyangconn/music-online-go/internal/config"
	"github.com/kyangconn/music-online-go/internal/domain"
)

func TestPostgresDSNEncodesCredentialsAndOptions(t *testing.T) {
	cfg := config.DatabaseConfig{
		Host:                  "2001:db8::1",
		Port:                  "5432",
		User:                  "music user",
		Password:              "p@ss word/with:symbols",
		Name:                  "music/db",
		SSLMode:               "verify-full",
		ConnectTimeoutSeconds: 12,
	}
	parsed, err := url.Parse(postgresDSN(cfg))
	if err != nil {
		t.Fatalf("parse postgres DSN: %v", err)
	}
	if got := parsed.Host; got != "[2001:db8::1]:5432" {
		t.Fatalf("host = %q", got)
	}
	if got := parsed.User.Username(); got != cfg.User {
		t.Fatalf("user = %q", got)
	}
	if got, ok := parsed.User.Password(); !ok || got != cfg.Password {
		t.Fatalf("password = %q, present = %v", got, ok)
	}
	if got := parsed.Path; got != "/"+cfg.Name {
		t.Fatalf("path = %q", got)
	}
	if got := parsed.Query().Get("sslmode"); got != cfg.SSLMode {
		t.Fatalf("sslmode = %q", got)
	}
	if got := parsed.Query().Get("connect_timeout"); got != "12" {
		t.Fatalf("connect_timeout = %q", got)
	}
}

func TestConnectionPoolLimits(t *testing.T) {
	tests := []struct {
		name     string
		cfg      config.DatabaseConfig
		wantOpen int
		wantIdle int
	}{
		{name: "sqlite defaults", cfg: config.DatabaseConfig{Type: "sqlite"}, wantOpen: 1, wantIdle: 1},
		{name: "postgres defaults", cfg: config.DatabaseConfig{Type: "postgres"}, wantOpen: 25, wantIdle: 5},
		{name: "explicit values", cfg: config.DatabaseConfig{Type: "postgres", MaxOpenConnections: 12, MaxIdleConnections: 4}, wantOpen: 12, wantIdle: 4},
		{name: "idle is clamped", cfg: config.DatabaseConfig{Type: "postgres", MaxOpenConnections: 2}, wantOpen: 2, wantIdle: 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			open, idle := connectionPoolLimits(tt.cfg)
			if open != tt.wantOpen || idle != tt.wantIdle {
				t.Fatalf("limits = (%d, %d), want (%d, %d)", open, idle, tt.wantOpen, tt.wantIdle)
			}
		})
	}
}

func TestGORMLogLevel(t *testing.T) {
	tests := []struct {
		configured string
		mode       string
		want       logger.LogLevel
	}{
		{configured: "auto", mode: "debug", want: logger.Info},
		{configured: "auto", mode: "release", want: logger.Warn},
		{configured: "silent", mode: "debug", want: logger.Silent},
		{configured: "error", mode: "release", want: logger.Error},
		{configured: "warn", mode: "debug", want: logger.Warn},
		{configured: "info", mode: "release", want: logger.Info},
	}
	for _, tt := range tests {
		if got := gormLogLevel(tt.configured, tt.mode); got != tt.want {
			t.Fatalf("gormLogLevel(%q, %q) = %v, want %v", tt.configured, tt.mode, got, tt.want)
		}
	}
}

func TestGORMLoggerRedactsQueryParameters(t *testing.T) {
	configured := newGORMLogger(logger.Info)
	filter, ok := configured.(gorm.ParamsFilter)
	if !ok {
		t.Fatal("configured GORM logger does not support parameter filtering")
	}

	query, params := filter.ParamsFilter(context.Background(), "SELECT * FROM users WHERE totp_secret = ?", "sensitive-value")
	if query != "SELECT * FROM users WHERE totp_secret = ?" {
		t.Fatalf("query template changed unexpectedly: %q", query)
	}
	if params != nil {
		t.Fatalf("parameterized logger retained sensitive parameters: %#v", params)
	}
}

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
	for _, table := range []string{
		"users", "vinyl", "user_music_likes", "media_library_roots", "media_library_root_states", "media_files", "media_scan_jobs", "media_scan_issues",
		"music_artist_credits", "music_album_memberships", "music_genre_facets", "playlists", "playlist_items", "schema_migrations",
	} {
		if !db.Migrator().HasTable(table) {
			t.Fatalf("expected table %q to exist", table)
		}
	}
	if db.Migrator().HasTable("music_tags") || db.Migrator().HasTable("legacy_music_tags_v1") {
		t.Fatal("fresh database should not retain a legacy metadata table")
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

func TestLegacyManagedRelativePathPreservesNestedUploadLocation(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "42", "audio.flac")
	relative, err := legacyManagedRelativePath(root, path)
	if err != nil {
		t.Fatalf("derive managed relative path: %v", err)
	}
	if relative != "42/audio.flac" {
		t.Fatalf("relative path = %q, want nested upload location", relative)
	}
	if _, err := legacyManagedRelativePath(root, filepath.Join(t.TempDir(), "audio.flac")); err == nil {
		t.Fatal("path outside the configured upload root must not be guessed")
	}
}

func TestUnifyMusicMetadataMergesOnlyUnambiguousLegacyTagsAndArchivesSource(t *testing.T) {
	db := openTestDatabase(t)
	if err := migrate(db, migrations[:2]); err != nil {
		t.Fatalf("prepare legacy schema: %v", err)
	}

	user := domain.User{Username: "migration-owner", Email: "migration@example.com", Password: "unused"}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
	music := legacyMusicV1{Title: "Track", Artist: "Artist", Album: "Release", UserID: user.ID}
	if err := db.Create(&music).Error; err != nil {
		t.Fatalf("create music: %v", err)
	}
	tags := []legacyMusicTagV1{
		{
			Title: "track", Artist: "artist", Album: "release", AlbumArtist: "Album Artist",
			Genre: "Ambient; Chillout", TrackNumber: 2, DiscNumber: 1,
			MusicBrainzID: "123e4567-e89b-42d3-a456-426614174000",
		},
		{Title: "Unmatched", Artist: "Nobody", Genre: "Archived Only", UseCount: 9},
	}
	if err := db.Create(&tags).Error; err != nil {
		t.Fatalf("create legacy tags: %v", err)
	}

	if err := migrate(db, migrations); err != nil {
		t.Fatalf("apply canonical metadata migration: %v", err)
	}
	if db.Migrator().HasTable("music_tags") || !db.Migrator().HasTable("legacy_music_tags_v1") {
		t.Fatal("legacy table was not archived")
	}

	var migrated domain.Music
	if err := db.First(&migrated, music.ID).Error; err != nil {
		t.Fatalf("load migrated music: %v", err)
	}
	if migrated.AlbumArtist != "Album Artist" || migrated.TrackNumber != 2 || migrated.DiscNumber != 1 {
		t.Fatalf("legacy metadata was not merged: %+v", migrated)
	}
	if migrated.MusicBrainzRecordingID != "123e4567-e89b-42d3-a456-426614174000" {
		t.Fatalf("recording ID = %q", migrated.MusicBrainzRecordingID)
	}
	if len(migrated.Genres) != 2 || migrated.Genres[0] != "Ambient" || migrated.Genres[1] != "Chillout" {
		t.Fatalf("genres = %#v", migrated.Genres)
	}
	if len(migrated.GenreTokens) != 2 || migrated.GenreTokens[0] != "ambient" || migrated.GenreTokens[1] != "chillout" {
		t.Fatalf("genre tokens = %#v", migrated.GenreTokens)
	}
	var artistCredits []domain.MusicArtistCredit
	if err := db.Where("music_id = ?", migrated.ID).Find(&artistCredits).Error; err != nil || len(artistCredits) == 0 {
		t.Fatalf("browse artist projection was not backfilled: credits=%+v err=%v", artistCredits, err)
	}
	var albumMembership domain.MusicAlbumMembership
	if err := db.First(&albumMembership, "music_id = ?", migrated.ID).Error; err != nil ||
		albumMembership.Title != "Release" || albumMembership.NormalizedTitle != "release" {
		t.Fatalf("browse album projection was not backfilled: membership=%+v err=%v", albumMembership, err)
	}
	var genreFacets []domain.MusicGenreFacet
	if err := db.Where("music_id = ?", migrated.ID).Order("position ASC").Find(&genreFacets).Error; err != nil || len(genreFacets) != 2 {
		t.Fatalf("browse genre projection was not backfilled: facets=%+v err=%v", genreFacets, err)
	}

	var archivedCount int64
	if err := db.Table("legacy_music_tags_v1").Count(&archivedCount).Error; err != nil {
		t.Fatalf("count archived tags: %v", err)
	}
	if archivedCount != int64(len(tags)) {
		t.Fatalf("archived tags = %d, want %d", archivedCount, len(tags))
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
	for _, index := range []string{"idx_analysis_claim", "idx_analysis_music_kind_id"} {
		if !DB.Migrator().HasIndex(&domain.MusicAnalysisJob{}, index) {
			t.Fatalf("music analysis index %q was not created", index)
		}
	}
}
