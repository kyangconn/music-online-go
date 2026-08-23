package database

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/kyangconn/music-online-go/internal/config"
	"github.com/kyangconn/music-online-go/internal/domain"
	pklog "github.com/kyangconn/music-online-go/internal/pkg/log"
)

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
