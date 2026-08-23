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
	"strings"

	"gorm.io/gorm"

	"github.com/kyangconn/music-online-go/internal/domain"
	pklog "github.com/kyangconn/music-online-go/internal/pkg/log"
)

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
