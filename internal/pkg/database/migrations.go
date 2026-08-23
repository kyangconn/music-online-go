package database

import (
	"errors"
	"fmt"
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

type legacyUserV1 struct {
	ID          uint `gorm:"primarykey"`
	CreatedAt   time.Time
	UpdatedAt   time.Time
	DeletedAt   gorm.DeletedAt `gorm:"index"`
	Username    string         `gorm:"uniqueIndex;size:100;not null"`
	Email       string         `gorm:"uniqueIndex;size:255;not null"`
	Password    string         `gorm:"size:255;not null"`
	FullName    string         `gorm:"size:255"`
	Nickname    string         `gorm:"size:100"`
	AvatarURL   string         `gorm:"size:500"`
	Phone       string         `gorm:"size:20"`
	Bio         string         `gorm:"type:text"`
	IsActive    bool           `gorm:"default:true"`
	IsVerified  bool           `gorm:"default:false"`
	Role        string         `gorm:"size:50;default:'user'"`
	TOTPSecret  string         `gorm:"size:100"`
	TOTPEnabled bool           `gorm:"default:false"`
}

func (*legacyUserV1) TableName() string {
	return "users"
}

// legacyMusicV1 freezes the vinyl schema that migration 1 originally created.
// Fresh installations replay that historical shape before migration 3 adds the
// canonical metadata columns, so an old and a new database follow the same
// deterministic upgrade path.
type legacyMusicV1 struct {
	ID          uint `gorm:"primarykey"`
	CreatedAt   time.Time
	UpdatedAt   time.Time
	DeletedAt   gorm.DeletedAt `gorm:"index"`
	Title       string         `gorm:"size:255;not null;index"`
	Artist      string         `gorm:"size:255;not null;index"`
	Album       string         `gorm:"size:255;index"`
	Year        int            `gorm:"index"`
	TrackNumber int
	Genre       string `gorm:"size:100;index"`
	Duration    int
	Intro       string `gorm:"type:text"`
	Img         string `gorm:"size:500"`
	Path        string `gorm:"size:500"`
	FileHash    string `gorm:"size:64;index"`
	Type        string `gorm:"size:20;default:'single'"`
	IssuingDate time.Time
	UserID      uint  `gorm:"index"`
	AlbumID     *uint `gorm:"index"`
}

func (*legacyMusicV1) TableName() string {
	return "vinyl"
}

type legacyUserMusicLikeV1 struct {
	UserID    uint `gorm:"primaryKey"`
	MusicID   uint `gorm:"primaryKey"`
	CreatedAt time.Time
}

func (*legacyUserMusicLikeV1) TableName() string {
	return "user_music_likes"
}

// legacyMusicTagV1 describes the table created by migration 1. It remains
// migration-local so the application cannot accidentally reintroduce a second
// runtime metadata model after migration 3 archives the table.
type legacyMusicTagV1 struct {
	ID                  uint `gorm:"primarykey"`
	CreatedAt           time.Time
	UpdatedAt           time.Time
	DeletedAt           gorm.DeletedAt `gorm:"index"`
	Artist              string         `gorm:"size:255;index:idx_artist_title;index:idx_artist"`
	Title               string         `gorm:"size:255;index:idx_artist_title;index:idx_title"`
	Album               string         `gorm:"size:255;index:idx_album"`
	AlbumArtist         string         `gorm:"size:255;index:idx_album_artist"`
	TrackNumber         int            `gorm:"index:idx_track_number"`
	DiscNumber          int            `gorm:"index:idx_disc_number"`
	Genre               string         `gorm:"size:100;index:idx_genre"`
	Year                int            `gorm:"index:idx_year"`
	Duration            int            `gorm:"index:idx_duration"`
	Comment             string         `gorm:"type:text"`
	MusicBrainzID       string         `gorm:"size:36;index:idx_musicbrainz_id"`
	MusicBrainzArtistID string         `gorm:"size:36;index:idx_musicbrainz_artist_id"`
	UseCount            int            `gorm:"default:0"`
	SearchVector        string         `gorm:"type:text"`
}

func (*legacyMusicTagV1) TableName() string {
	return "music_tags"
}

// Keep this list append-only. Applied versions must never be reordered,
// renamed, or reused; every schema change gets a new, higher version.
var migrations = []migration{
	{
		version: 1,
		name:    "initial_schema",
		up: func(db *gorm.DB) error {
			return db.AutoMigrate(
				&legacyUserV1{},
				&legacyMusicV1{},
				&legacyUserMusicLikeV1{},
				&legacyMusicTagV1{},
			)
		},
	},
	{
		version: 2,
		name:    "backfill_music_file_hashes",
		up:      backfillMusicFileHashes,
	},
	{
		version: 3,
		name:    "unify_music_metadata",
		up:      unifyMusicMetadata,
	},
	{
		version: 4,
		name:    "add_server_media_libraries",
		up: func(db *gorm.DB) error {
			if err := db.AutoMigrate(
				&domain.MediaLibraryRoot{},
				&domain.MediaScanJob{},
				&domain.MediaScanIssue{},
				&domain.Music{},
			); err != nil {
				return fmt.Errorf("add server media library schema: %w", err)
			}
			return nil
		},
	},
	{
		version: 5,
		name:    "stabilize_media_storage",
		up:      stabilizeMediaStorage,
	},
	{
		version: 6,
		name:    "add_library_browse_and_playlists",
		up:      addLibraryBrowseAndPlaylists,
	},
	{
		version: 7,
		name:    "add_music_preset_classification",
		up: func(db *gorm.DB) error {
			if err := db.AutoMigrate(
				&domain.MusicPresetClassification{},
				&domain.MusicPresetScore{},
			); err != nil {
				return fmt.Errorf("add preset classification schema: %w", err)
			}
			return nil
		},
	},
	{
		version: 8,
		name:    "add_music_analysis_jobs",
		up: func(db *gorm.DB) error {
			if err := db.AutoMigrate(
				&domain.MusicAudioAnalysis{},
				&domain.MusicAnalysisJob{},
			); err != nil {
				return fmt.Errorf("add music analysis schema: %w", err)
			}
			return nil
		},
	},
	{
		version: 9,
		name:    "index_music_analysis_queue",
		up: func(db *gorm.DB) error {
			for _, name := range []string{"idx_analysis_claim", "idx_analysis_music_kind_id"} {
				if db.Migrator().HasIndex(&domain.MusicAnalysisJob{}, name) {
					continue
				}
				if err := db.Migrator().CreateIndex(&domain.MusicAnalysisJob{}, name); err != nil {
					return fmt.Errorf("create music analysis index %s: %w", name, err)
				}
			}
			return nil
		},
	},
	{
		version: 10,
		name:    "link_preset_audio_analysis",
		up: func(db *gorm.DB) error {
			if err := db.AutoMigrate(&domain.MusicPresetClassification{}); err != nil {
				return fmt.Errorf("link preset classification to audio analysis: %w", err)
			}
			return nil
		},
	},
	{
		version: 11,
		name:    "add_user_sessions",
		up: func(db *gorm.DB) error {
			if err := db.AutoMigrate(&domain.Session{}); err != nil {
				return fmt.Errorf("add user session schema: %w", err)
			}
			return nil
		},
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
