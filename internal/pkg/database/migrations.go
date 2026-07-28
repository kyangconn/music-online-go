package database

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/kyangconn/music-online-go/internal/config"
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
}

func addLibraryBrowseAndPlaylists(db *gorm.DB) error {
	if err := db.AutoMigrate(
		&domain.MusicArtistCredit{},
		&domain.MusicAlbumMembership{},
		&domain.MusicGenreFacet{},
		&domain.Playlist{},
		&domain.PlaylistItem{},
	); err != nil {
		return fmt.Errorf("add library browse and playlist schema: %w", err)
	}

	const batchSize = 200
	var lastID uint
	for {
		var musics []*domain.Music
		if err := db.Where("id > ?", lastID).Order("id ASC").Limit(batchSize).Find(&musics).Error; err != nil {
			return fmt.Errorf("load music for browse projection backfill: %w", err)
		}
		for _, music := range musics {
			genreValues := music.Genres
			if len(genreValues) == 0 && strings.TrimSpace(music.Genre) != "" {
				genreValues = domain.StringList{music.Genre}
			}
			normalizedGenreTokens := domain.NormalizeGenreTokens(genreValues)
			if !slices.Equal(music.GenreTokens, normalizedGenreTokens) {
				if err := db.Model(&domain.Music{}).Where("id = ?", music.ID).
					Update("genre_tokens", normalizedGenreTokens).Error; err != nil {
					return fmt.Errorf("normalize genre tokens for music %d: %w", music.ID, err)
				}
				music.GenreTokens = normalizedGenreTokens
			}
			projection := domain.BuildMusicBrowseProjection(music)
			if len(projection.ArtistCredits) > 0 {
				if err := db.Create(&projection.ArtistCredits).Error; err != nil {
					return fmt.Errorf("backfill artist browse credits for music %d: %w", music.ID, err)
				}
			}
			if projection.AlbumMembership != nil {
				if err := db.Create(projection.AlbumMembership).Error; err != nil {
					return fmt.Errorf("backfill album browse membership for music %d: %w", music.ID, err)
				}
			}
			if len(projection.GenreFacets) > 0 {
				if err := db.Create(&projection.GenreFacets).Error; err != nil {
					return fmt.Errorf("backfill genre browse facets for music %d: %w", music.ID, err)
				}
			}
			lastID = music.ID
		}
		if len(musics) < batchSize {
			break
		}
	}
	return nil
}

func stabilizeMediaStorage(db *gorm.DB) error {
	if err := db.AutoMigrate(
		&domain.MediaLibraryRoot{},
		&domain.MediaLibraryRootState{},
		&domain.MediaFile{},
		&domain.MediaScanJob{},
		&domain.MediaScanIssue{},
	); err != nil {
		return fmt.Errorf("add stable media storage schema: %w", err)
	}

	var roots []*domain.MediaLibraryRoot
	if err := db.Unscoped().Where("key = ?", "").Find(&roots).Error; err != nil {
		return fmt.Errorf("load media root keys: %w", err)
	}
	for _, root := range roots {
		key := fmt.Sprintf("root-%d", root.ID)
		if err := db.Unscoped().Model(&domain.MediaLibraryRoot{}).Where("id = ?", root.ID).Update("key", key).Error; err != nil {
			return fmt.Errorf("backfill media root key %d: %w", root.ID, err)
		}
	}
	// The partial predicate keeps an interrupted legacy row with an empty key
	// from blocking recovery while all application-created roots remain unique.
	if err := db.Exec("CREATE UNIQUE INDEX IF NOT EXISTS idx_media_library_roots_key_unique ON media_library_roots(\"key\") WHERE \"key\" <> ''").Error; err != nil {
		return fmt.Errorf("create media root key constraint: %w", err)
	}

	if err := retireDuplicateActiveScans(db); err != nil {
		return err
	}
	if err := db.Exec("CREATE UNIQUE INDEX IF NOT EXISTS idx_media_scan_jobs_active_root ON media_scan_jobs(root_id) WHERE status IN ('pending', 'running', 'retry_wait')").Error; err != nil {
		return fmt.Errorf("create active media scan constraint: %w", err)
	}
	return backfillMediaFiles(db)
}

func retireDuplicateActiveScans(db *gorm.DB) error {
	var active []*domain.MediaScanJob
	if err := db.Where("status IN ?", []string{domain.MediaScanStatusPending, domain.MediaScanStatusRunning, domain.MediaScanStatusRetryWait}).
		Order("root_id ASC, id DESC").Find(&active).Error; err != nil {
		return fmt.Errorf("load active media scans: %w", err)
	}
	keptRoots := make(map[uint]struct{})
	now := time.Now().UTC()
	for _, job := range active {
		if _, exists := keptRoots[job.RootID]; !exists {
			keptRoots[job.RootID] = struct{}{}
			continue
		}
		if err := db.Model(&domain.MediaScanJob{}).Where("id = ?", job.ID).Updates(map[string]interface{}{
			"status":        domain.MediaScanStatusFailed,
			"finished_at":   now,
			"error_summary": "superseded while upgrading duplicate active scan records",
		}).Error; err != nil {
			return fmt.Errorf("retire duplicate media scan %d: %w", job.ID, err)
		}
	}
	return nil
}

func backfillMediaFiles(db *gorm.DB) error {
	managedRoot := "uploads"
	if config.AppConfig != nil && strings.TrimSpace(config.AppConfig.Server.UploadDir) != "" {
		managedRoot = config.AppConfig.Server.UploadDir
	}
	var musics []*domain.Music
	if err := db.Where("media_relative_path <> ? OR path <> ?", "", "").Find(&musics).Error; err != nil {
		return fmt.Errorf("load legacy media sources: %w", err)
	}
	for _, music := range musics {
		relative := strings.TrimSpace(music.MediaRelativePath)
		if relative == "" {
			if music.MediaRootID != domain.ManagedMediaRootID {
				continue
			}
			var err error
			relative, err = legacyManagedRelativePath(managedRoot, music.Path)
			if err != nil {
				// Preserve the legacy direct-path fallback. Creating a guessed
				// MediaFile would make a previously valid row point elsewhere.
				pklog.Warnf("Skipping media source backfill for music %d: %v", music.ID, err)
				continue
			}
		}
		if relative == "" || relative == "." {
			continue
		}
		sourceKey := ""
		if music.MediaSourceKey != nil {
			sourceKey = strings.TrimSpace(*music.MediaSourceKey)
		}
		if sourceKey == "" {
			identity := fmt.Sprintf("%d\x00%s", music.MediaRootID, relative)
			if runtime.GOOS == "windows" {
				identity = strings.ToLower(identity)
			}
			sum := sha256.Sum256([]byte(identity))
			sourceKey = hex.EncodeToString(sum[:])
		}
		seenAt := music.UpdatedAt.UTC()
		if seenAt.IsZero() {
			seenAt = time.Now().UTC()
		}
		mediaFile := &domain.MediaFile{
			MusicID:          music.ID,
			RootID:           music.MediaRootID,
			RelativePath:     relative,
			SourceKey:        sourceKey,
			FileHash:         music.FileHash,
			ObservedFileHash: music.FileHash,
			FileSize:         music.SourceFileSize,
			FileModTime:      music.SourceFileModTime,
			ReadOnly:         music.SourceReadOnly,
			Availability:     domain.MediaFileAvailabilityUnknown,
			ContentRevision:  1,
			LastSeenAt:       &seenAt,
		}
		if err := db.Where("source_key = ?", sourceKey).FirstOrCreate(mediaFile).Error; err != nil {
			return fmt.Errorf("backfill media file for music %d: %w", music.ID, err)
		}
		if mediaFile.MusicID != music.ID {
			// Two legacy rows may reference the same physical path. Keep the
			// first authoritative source instead of violating source identity.
			continue
		}
		if strings.TrimSpace(music.MediaRelativePath) == "" || music.MediaSourceKey == nil {
			if err := db.Model(&domain.Music{}).Where("id = ?", music.ID).Updates(map[string]interface{}{
				"media_relative_path": relative,
				"media_source_key":    sourceKey,
			}).Error; err != nil {
				return fmt.Errorf("backfill media provenance for music %d: %w", music.ID, err)
			}
		}
	}
	return nil
}

func legacyManagedRelativePath(root, storedPath string) (string, error) {
	if strings.TrimSpace(root) == "" || strings.TrimSpace(storedPath) == "" {
		return "", errors.New("managed upload path is empty")
	}
	rootAbs, err := filepath.Abs(filepath.Clean(root))
	if err != nil {
		return "", fmt.Errorf("resolve managed upload root: %w", err)
	}
	candidateAbs, err := filepath.Abs(filepath.Clean(storedPath))
	if err != nil {
		return "", fmt.Errorf("resolve legacy media path: %w", err)
	}
	relative, err := filepath.Rel(rootAbs, candidateAbs)
	if err != nil || relative == "." || filepath.IsAbs(relative) || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", errors.New("legacy media path is outside server.upload_dir")
	}
	return filepath.ToSlash(relative), nil
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

var (
	legacyGenreSeparator = regexp.MustCompile(`[;,/]+`)
	legacyMBIDPattern    = regexp.MustCompile(`(?i)^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
)

func unifyMusicMetadata(db *gorm.DB) error {
	if err := db.AutoMigrate(&domain.Music{}); err != nil {
		return fmt.Errorf("add canonical music metadata columns: %w", err)
	}

	var musics []*domain.Music
	if err := db.Unscoped().Find(&musics).Error; err != nil {
		return fmt.Errorf("load music metadata for migration: %w", err)
	}
	for _, music := range musics {
		backfillCanonicalMusicMetadata(music)
		if err := db.Unscoped().Save(music).Error; err != nil {
			return fmt.Errorf("backfill canonical metadata for music %d: %w", music.ID, err)
		}
	}

	if !db.Migrator().HasTable(&legacyMusicTagV1{}) {
		return nil
	}
	var legacyTags []*legacyMusicTagV1
	if err := db.Unscoped().Find(&legacyTags).Error; err != nil {
		return fmt.Errorf("load legacy music tags: %w", err)
	}
	for _, tag := range legacyTags {
		if tag.DeletedAt.Valid {
			continue
		}
		music, err := findUnambiguousLegacyTagTarget(db, tag)
		if err != nil {
			return err
		}
		if music == nil {
			continue
		}
		if !mergeLegacyMusicTag(music, tag) {
			continue
		}
		backfillCanonicalMusicMetadata(music)
		if err := db.Unscoped().Save(music).Error; err != nil {
			return fmt.Errorf("merge legacy music tag %d into music %d: %w", tag.ID, music.ID, err)
		}
	}
	if len(legacyTags) == 0 {
		if err := db.Migrator().DropTable(&legacyMusicTagV1{}); err != nil {
			return fmt.Errorf("remove empty legacy music_tags table: %w", err)
		}
		return nil
	}

	const legacyTableName = "legacy_music_tags_v1"
	if db.Migrator().HasTable(legacyTableName) {
		return fmt.Errorf("cannot archive music_tags: %s already exists", legacyTableName)
	}
	if err := db.Migrator().RenameTable("music_tags", legacyTableName); err != nil {
		return fmt.Errorf("archive legacy music_tags table: %w", err)
	}
	return nil
}

func backfillCanonicalMusicMetadata(music *domain.Music) {
	if len(music.Artists) == 0 && strings.TrimSpace(music.Artist) != "" {
		music.Artists = domain.StringList{strings.TrimSpace(music.Artist)}
	}
	if len(music.AlbumArtists) == 0 && strings.TrimSpace(music.AlbumArtist) != "" {
		music.AlbumArtists = domain.StringList{strings.TrimSpace(music.AlbumArtist)}
	}
	if len(music.Genres) == 0 {
		music.Genres = splitLegacyGenres(music.Genre)
	}
	if len(music.GenreTokens) == 0 {
		music.GenreTokens = domain.StringList{}
		seen := make(map[string]struct{})
		for _, genre := range music.Genres {
			for _, part := range legacyGenreSeparator.Split(genre, -1) {
				token := strings.ToLower(strings.Join(strings.Fields(part), " "))
				if token == "" {
					continue
				}
				if _, exists := seen[token]; exists {
					continue
				}
				seen[token] = struct{}{}
				music.GenreTokens = append(music.GenreTokens, token)
			}
		}
	}
	if music.ReleaseDate == "" && !music.IssuingDate.IsZero() {
		music.ReleaseDate = music.IssuingDate.Format("2006-01-02")
	}
	if music.MetadataRevision == 0 {
		music.MetadataRevision = 1
	}
}

func splitLegacyGenres(value string) domain.StringList {
	parts := legacyGenreSeparator.Split(value, -1)
	genres := make(domain.StringList, 0, len(parts))
	seen := make(map[string]struct{})
	for _, part := range parts {
		genre := strings.TrimSpace(part)
		key := strings.ToLower(genre)
		if genre == "" {
			continue
		}
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		genres = append(genres, genre)
	}
	return genres
}

func findUnambiguousLegacyTagTarget(db *gorm.DB, tag *legacyMusicTagV1) (*domain.Music, error) {
	var candidates []*domain.Music
	query := db.Unscoped().Where("LOWER(title) = LOWER(?) AND LOWER(artist) = LOWER(?)", tag.Title, tag.Artist)
	if strings.TrimSpace(tag.Album) != "" {
		query = query.Where("LOWER(album) = LOWER(?)", tag.Album)
	}
	if err := query.Limit(2).Find(&candidates).Error; err != nil {
		return nil, fmt.Errorf("match legacy music tag %d: %w", tag.ID, err)
	}
	if len(candidates) != 1 {
		return nil, nil
	}
	return candidates[0], nil
}

func mergeLegacyMusicTag(music *domain.Music, tag *legacyMusicTagV1) bool {
	changed := false
	if music.Album == "" {
		music.Album = tag.Album
		changed = changed || tag.Album != ""
	}
	if music.AlbumArtist == "" {
		music.AlbumArtist = tag.AlbumArtist
		changed = changed || tag.AlbumArtist != ""
	}
	if music.TrackNumber == 0 {
		music.TrackNumber = tag.TrackNumber
		changed = changed || tag.TrackNumber != 0
	}
	if music.DiscNumber == 0 {
		music.DiscNumber = tag.DiscNumber
		changed = changed || tag.DiscNumber != 0
	}
	if music.Genre == "" {
		music.Genre = tag.Genre
		music.Genres = nil
		music.GenreTokens = nil
		changed = changed || tag.Genre != ""
	}
	if music.Year == 0 {
		music.Year = tag.Year
		changed = changed || tag.Year != 0
	}
	if music.Duration == 0 {
		music.Duration = tag.Duration
		changed = changed || tag.Duration != 0
	}
	if music.Comment == "" {
		music.Comment = tag.Comment
		changed = changed || tag.Comment != ""
	}
	legacyRecordingID := strings.ToLower(strings.TrimSpace(tag.MusicBrainzID))
	if music.MusicBrainzRecordingID == "" && legacyMBIDPattern.MatchString(legacyRecordingID) {
		music.MusicBrainzRecordingID = legacyRecordingID
		changed = true
	}
	legacyArtistID := strings.ToLower(strings.TrimSpace(tag.MusicBrainzArtistID))
	if len(music.MusicBrainzArtistIDs) == 0 && legacyMBIDPattern.MatchString(legacyArtistID) {
		music.MusicBrainzArtistIDs = domain.StringList{legacyArtistID}
		changed = true
	}
	if changed {
		music.MetadataRevision++
	}
	return changed
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
