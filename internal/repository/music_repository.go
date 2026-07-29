// Package repository music_repository.go - 音乐仓库层
// 音乐实体的增删改查，支持分页、搜索、权限过滤
package repository

import (
	"context"
	"errors"

	"github.com/kyangconn/music-online-go/internal/domain"
	"gorm.io/gorm"
)

var (
	ErrMusicNotFound = errors.New("music not found")
)

type MusicRepository interface {
	Create(ctx context.Context, music *domain.Music) error
	FindByID(ctx context.Context, id uint) (*domain.Music, error)
	FindByIDs(ctx context.Context, ids []uint) ([]*domain.Music, error)
	FindByFileHash(ctx context.Context, fileHash string) (*domain.Music, error)
	FindByMediaSourceKey(ctx context.Context, sourceKey string) (*domain.Music, error)
	FindByTitleAndArtist(ctx context.Context, title, artist string, limit int) ([]*domain.Music, error)
	FindByStableMetadataIDs(ctx context.Context, recordingID, trackID string, limit int) ([]*domain.Music, error)
	FindByMusicBrainzRecordingID(ctx context.Context, recordingID string) (*domain.Music, error)
	Search(ctx context.Context, params *domain.MusicSearchParams) ([]*domain.Music, int64, error)
	ListFilterOptions(ctx context.Context) (*domain.MusicFilterOptions, error)
	CountWithMetadata(ctx context.Context) (int64, error)
	ListByUserID(ctx context.Context, userID uint, page, pageSize int) ([]*domain.Music, int64, error)
	Update(ctx context.Context, music *domain.Music) error
	Delete(ctx context.Context, id uint) error

	// Like/Collection
	LikeMusic(ctx context.Context, userID, musicID uint) error
	UnlikeMusic(ctx context.Context, userID, musicID uint) error
	IsLiked(ctx context.Context, userID, musicID uint) (bool, error)
	CountLikes(ctx context.Context, musicID uint) (int64, error)
	ListEngagementByMusicIDs(ctx context.Context, musicIDs []uint, currentUserID *uint) (map[uint]MusicEngagement, error)
	ListLikedByUserID(ctx context.Context, userID uint, page, pageSize int) ([]*domain.Music, int64, error)
}

type MusicEngagement struct {
	LikeCount int64
	IsLiked   bool
}

type musicRepository struct {
	db           *gorm.DB
	presetPolicy domain.PresetRulePolicy
}

func NewMusicRepository(db *gorm.DB, presetPolicies ...domain.PresetRulePolicy) MusicRepository {
	var presetPolicy domain.PresetRulePolicy
	if len(presetPolicies) > 0 {
		presetPolicy = presetPolicies[0]
	}
	return &musicRepository{db: db, presetPolicy: presetPolicy}
}

func (r *musicRepository) Create(ctx context.Context, music *domain.Music) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(music).Error; err != nil {
			return err
		}
		if err := replaceMusicBrowseProjection(tx, music); err != nil {
			return err
		}
		return replaceMusicPresetProjection(tx, music, r.presetPolicy)
	})
}

func (r *musicRepository) FindByID(ctx context.Context, id uint) (*domain.Music, error) {
	var music domain.Music
	if err := r.db.WithContext(ctx).First(&music, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrMusicNotFound
		}
		return nil, err
	}
	return &music, nil
}

func (r *musicRepository) FindByIDs(ctx context.Context, ids []uint) ([]*domain.Music, error) {
	if len(ids) == 0 {
		return []*domain.Music{}, nil
	}
	var rows []*domain.Music
	if err := r.db.WithContext(ctx).Where("id IN ?", ids).Find(&rows).Error; err != nil {
		return nil, err
	}
	byID := make(map[uint]*domain.Music, len(rows))
	for _, music := range rows {
		byID[music.ID] = music
	}
	ordered := make([]*domain.Music, 0, len(rows))
	for _, id := range ids {
		if music := byID[id]; music != nil {
			ordered = append(ordered, music)
		}
	}
	return ordered, nil
}

func (r *musicRepository) FindByFileHash(ctx context.Context, fileHash string) (*domain.Music, error) {
	var music domain.Music
	if err := r.db.WithContext(ctx).Where("file_hash = ?", fileHash).First(&music).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrMusicNotFound
		}
		return nil, err
	}
	return &music, nil
}

func (r *musicRepository) FindByMediaSourceKey(ctx context.Context, sourceKey string) (*domain.Music, error) {
	var music domain.Music
	if err := r.db.WithContext(ctx).Where("media_source_key = ?", sourceKey).First(&music).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrMusicNotFound
		}
		return nil, err
	}
	return &music, nil
}

func (r *musicRepository) FindByTitleAndArtist(ctx context.Context, title, artist string, limit int) ([]*domain.Music, error) {
	if limit <= 0 {
		limit = 5
	}
	var musics []*domain.Music
	err := r.db.WithContext(ctx).
		Where("LOWER(title) = LOWER(?) AND LOWER(artist) = LOWER(?)", title, artist).
		Order("created_at DESC").
		Limit(limit).
		Find(&musics).Error
	return musics, err
}

func (r *musicRepository) FindByStableMetadataIDs(ctx context.Context, recordingID, trackID string, limit int) ([]*domain.Music, error) {
	if limit <= 0 {
		limit = 5
	}
	results := make([]*domain.Music, 0, limit)
	seen := make(map[uint]struct{})
	appendMatches := func(column, value string) error {
		if value == "" || len(results) >= limit {
			return nil
		}
		var matches []*domain.Music
		if err := r.db.WithContext(ctx).
			Where(column+" = ?", value).
			Order("created_at DESC").
			Limit(limit - len(results)).
			Find(&matches).Error; err != nil {
			return err
		}
		for _, match := range matches {
			if _, exists := seen[match.ID]; exists {
				continue
			}
			seen[match.ID] = struct{}{}
			results = append(results, match)
		}
		return nil
	}
	if err := appendMatches("music_brainz_track_id", trackID); err != nil {
		return nil, err
	}
	if err := appendMatches("music_brainz_recording_id", recordingID); err != nil {
		return nil, err
	}
	return results, nil
}

func (r *musicRepository) FindByMusicBrainzRecordingID(ctx context.Context, recordingID string) (*domain.Music, error) {
	var music domain.Music
	if err := r.db.WithContext(ctx).Where("music_brainz_recording_id = ?", recordingID).First(&music).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrMusicNotFound
		}
		return nil, err
	}
	return &music, nil
}

func (r *musicRepository) Search(ctx context.Context, params *domain.MusicSearchParams) ([]*domain.Music, int64, error) {
	var musics []*domain.Music
	var total int64
	offset := (params.Page - 1) * params.PageSize
	if params.Offset != nil && *params.Offset >= 0 {
		offset = *params.Offset
	}

	db := r.db.WithContext(ctx).Model(&domain.Music{})
	if params.ArtistKey != "" {
		db = db.Joins("JOIN music_artist_credits ON music_artist_credits.music_id = vinyl.id AND music_artist_credits.group_key = ?", params.ArtistKey)
	}
	if params.AlbumKey != "" {
		db = db.Joins("JOIN music_album_memberships ON music_album_memberships.music_id = vinyl.id AND music_album_memberships.group_key = ?", params.AlbumKey)
	}
	if params.Query != "" {
		likeQuery := "%" + params.Query + "%"
		db = db.Where("(LOWER(vinyl.title) LIKE LOWER(?) OR LOWER(vinyl.artist) LIKE LOWER(?))", likeQuery, likeQuery)
	}
	if params.Title != "" {
		db = db.Where("LOWER(vinyl.title) = LOWER(?)", params.Title)
	}
	if params.Artist != "" {
		normalized := domain.NormalizeBrowseText(params.Artist)
		db = db.Where(`EXISTS (
			SELECT 1 FROM music_artist_credits AS artist_filter
			WHERE artist_filter.music_id = vinyl.id AND artist_filter.normalized_name_key = ?
				AND artist_filter.normalized_name = ? AND artist_filter.track_credit = ?
		)`, domain.NormalizedBrowseTextKey(normalized), normalized, true)
	}
	if params.Album != "" {
		normalized := domain.NormalizeBrowseText(params.Album)
		db = db.Where(`EXISTS (
			SELECT 1 FROM music_album_memberships AS album_filter
			WHERE album_filter.music_id = vinyl.id AND album_filter.normalized_title_key = ?
				AND album_filter.normalized_title = ?
		)`, domain.NormalizedBrowseTextKey(normalized), normalized)
	}
	if params.AlbumArtist != "" {
		normalized := domain.NormalizeBrowseText(params.AlbumArtist)
		db = db.Where(`EXISTS (
			SELECT 1 FROM music_album_memberships AS album_artist_filter
			WHERE album_artist_filter.music_id = vinyl.id AND album_artist_filter.normalized_album_artist_key = ?
				AND album_artist_filter.normalized_album_artist = ?
		)`, domain.NormalizedBrowseTextKey(normalized), normalized)
	}
	if params.Genre != "" {
		db = db.Joins("JOIN music_genre_facets ON music_genre_facets.music_id = vinyl.id AND music_genre_facets.normalized_name = ?", domain.NormalizeGenreName(params.Genre))
	}
	if params.Preset != "" || params.PresetStatus != "" {
		db = db.Joins("LEFT JOIN music_preset_classifications ON music_preset_classifications.music_id = vinyl.id")
		if params.Preset != "" {
			db = db.Where("COALESCE(music_preset_classifications.manual_preset, music_preset_classifications.automatic_preset) = ?", params.Preset)
		}
		if params.PresetStatus != "" {
			db = db.Where("COALESCE(music_preset_classifications.status, ?) = ?", domain.PresetStatusUnclassified, params.PresetStatus)
			if params.PresetStatus == domain.PresetStatusNeedsReview {
				db = db.Where("music_preset_classifications.manual_preset IS NULL")
			}
		}
	}
	if params.Year != nil {
		db = db.Where("vinyl.year = ?", *params.Year)
	}
	if params.MinYear != nil {
		db = db.Where("vinyl.year >= ?", *params.MinYear)
	}
	if params.MaxYear != nil {
		db = db.Where("vinyl.year <= ?", *params.MaxYear)
	}
	if params.Duration != nil {
		db = db.Where("vinyl.duration = ?", *params.Duration)
	}
	if params.MinDuration != nil {
		db = db.Where("vinyl.duration >= ?", *params.MinDuration)
	}
	if params.MaxDuration != nil {
		db = db.Where("vinyl.duration <= ?", *params.MaxDuration)
	}
	if params.RecordingID != "" {
		db = db.Where("vinyl.music_brainz_recording_id = ?", params.RecordingID)
	}
	if params.TrackID != "" {
		db = db.Where("vinyl.music_brainz_track_id = ?", params.TrackID)
	}
	if params.ReleaseID != "" {
		db = db.Where("vinyl.music_brainz_release_id = ?", params.ReleaseID)
	}
	if params.ReleaseGroupID != "" {
		db = db.Where("vinyl.music_brainz_release_group_id = ?", params.ReleaseGroupID)
	}
	if params.Type != nil {
		db = db.Where("vinyl.type = ?", *params.Type)
	}
	if params.LikedOnly {
		if params.LikedByUserID == nil {
			db = db.Where("1 = 0")
		} else {
			db = db.Joins("JOIN user_music_likes ON user_music_likes.music_id = vinyl.id").
				Where("user_music_likes.user_id = ?", *params.LikedByUserID)
		}
	}

	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	order := "vinyl.created_at DESC"
	if params.AlbumKey != "" {
		order = "CASE WHEN vinyl.disc_number > 0 THEN vinyl.disc_number ELSE 1 END ASC, " +
			"CASE WHEN vinyl.track_number > 0 THEN vinyl.track_number ELSE 2147483647 END ASC, vinyl.title ASC, vinyl.id ASC"
	}
	if err := db.Offset(offset).Limit(params.PageSize).Order(order).Find(&musics).Error; err != nil {
		return nil, 0, err
	}

	return musics, total, nil
}

func (r *musicRepository) ListFilterOptions(ctx context.Context) (*domain.MusicFilterOptions, error) {
	options := &domain.MusicFilterOptions{
		Artists:      []string{},
		Albums:       []string{},
		AlbumArtists: []string{},
		Genres:       []string{},
		Years:        []int{},
		Types:        []domain.MusicType{},
	}

	var rows []stringFilterOption
	if err := r.db.WithContext(ctx).Model(&domain.MusicArtistCredit{}).Where("track_credit = ?", true).
		Select("MIN(name) AS value").Group("normalized_name_key, normalized_name").
		Order("MIN(normalized_name) ASC").Scan(&rows).Error; err != nil {
		return nil, err
	}
	options.Artists = stringFilterOptionValues(rows)
	rows = nil
	if err := r.db.WithContext(ctx).Model(&domain.MusicAlbumMembership{}).
		Select("MIN(title) AS value").Group("normalized_title_key, normalized_title").
		Order("MIN(normalized_title) ASC").Scan(&rows).Error; err != nil {
		return nil, err
	}
	options.Albums = stringFilterOptionValues(rows)
	rows = nil
	if err := r.db.WithContext(ctx).Model(&domain.MusicAlbumMembership{}).
		Where("album_artist <> ''").Select("MIN(album_artist) AS value").
		Group("normalized_album_artist_key, normalized_album_artist").
		Order("MIN(normalized_album_artist) ASC").Scan(&rows).Error; err != nil {
		return nil, err
	}
	options.AlbumArtists = stringFilterOptionValues(rows)
	rows = nil
	if err := r.db.WithContext(ctx).Model(&domain.MusicGenreFacet{}).
		Select("MIN(display_name) AS value").Group("normalized_name").
		Order("normalized_name ASC").Scan(&rows).Error; err != nil {
		return nil, err
	}
	options.Genres = stringFilterOptionValues(rows)
	if err := r.db.WithContext(ctx).Model(&domain.Music{}).
		Where("year > 0").Distinct("year").Order("year DESC").Pluck("year", &options.Years).Error; err != nil {
		return nil, err
	}
	if err := r.db.WithContext(ctx).Model(&domain.Music{}).
		Where("type <> ''").Distinct("type").Order("type ASC").Pluck("type", &options.Types).Error; err != nil {
		return nil, err
	}

	return options, nil
}

type stringFilterOption struct {
	Value string
}

func stringFilterOptionValues(rows []stringFilterOption) []string {
	values := make([]string, 0, len(rows))
	for _, row := range rows {
		values = append(values, row.Value)
	}
	return values
}

func (r *musicRepository) CountWithMetadata(ctx context.Context) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&domain.Music{}).
		Where("music_brainz_recording_id <> ? OR music_brainz_release_id <> ? OR genre <> ? OR album_artist <> ?", "", "", "", "").
		Count(&count).Error
	return count, err
}

func (r *musicRepository) ListByUserID(ctx context.Context, userID uint, page, pageSize int) ([]*domain.Music, int64, error) {
	var musics []*domain.Music
	var total int64
	offset := (page - 1) * pageSize

	db := r.db.WithContext(ctx).Model(&domain.Music{}).Where("user_id = ?", userID)

	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	if err := db.Offset(offset).Limit(pageSize).Order("created_at desc").Find(&musics).Error; err != nil {
		return nil, 0, err
	}

	return musics, total, nil
}

func (r *musicRepository) Update(ctx context.Context, music *domain.Music) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Save(music).Error; err != nil {
			return err
		}
		if err := replaceMusicBrowseProjection(tx, music); err != nil {
			return err
		}
		return replaceMusicPresetProjection(tx, music, r.presetPolicy)
	})
}

func (r *musicRepository) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := deleteMusicAnalysisState(tx, []uint{id}); err != nil {
			return err
		}
		if err := deleteMusicBrowseProjection(tx, []uint{id}); err != nil {
			return err
		}
		if err := deleteMusicPresetProjection(tx, []uint{id}); err != nil {
			return err
		}
		if err := removeMusicFromPlaylists(tx, []uint{id}); err != nil {
			return err
		}
		if err := tx.Unscoped().Where("music_id = ?", id).Delete(&domain.MediaFile{}).Error; err != nil {
			return err
		}
		if err := tx.Where("music_id = ?", id).Delete(&domain.UserMusicLike{}).Error; err != nil {
			return err
		}
		if err := tx.Model(&domain.Music{}).Where("album_id = ?", id).Update("album_id", nil).Error; err != nil {
			return err
		}
		result := tx.Unscoped().Delete(&domain.Music{}, id)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return ErrMusicNotFound
		}
		return nil
	})
}

func (r *musicRepository) LikeMusic(ctx context.Context, userID, musicID uint) error {
	like := domain.UserMusicLike{
		UserID:  userID,
		MusicID: musicID,
	}
	// FirstOrCreate ensures idempotency
	return r.db.WithContext(ctx).FirstOrCreate(&like, like).Error
}

func (r *musicRepository) UnlikeMusic(ctx context.Context, userID, musicID uint) error {
	return r.db.WithContext(ctx).Where("user_id = ? AND music_id = ?", userID, musicID).Delete(&domain.UserMusicLike{}).Error
}

func (r *musicRepository) IsLiked(ctx context.Context, userID, musicID uint) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&domain.UserMusicLike{}).
		Where("user_id = ? AND music_id = ?", userID, musicID).
		Count(&count).Error
	return count > 0, err
}

func (r *musicRepository) CountLikes(ctx context.Context, musicID uint) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&domain.UserMusicLike{}).
		Where("music_id = ?", musicID).
		Count(&count).Error
	return count, err
}

func (r *musicRepository) ListEngagementByMusicIDs(ctx context.Context, musicIDs []uint, currentUserID *uint) (map[uint]MusicEngagement, error) {
	result := make(map[uint]MusicEngagement)
	if len(musicIDs) == 0 {
		return result, nil
	}
	type likeCountRow struct {
		MusicID   uint
		LikeCount int64
	}
	var counts []likeCountRow
	if err := r.db.WithContext(ctx).Model(&domain.UserMusicLike{}).
		Select("music_id, COUNT(*) AS like_count").Where("music_id IN ?", musicIDs).
		Group("music_id").Scan(&counts).Error; err != nil {
		return nil, err
	}
	for _, row := range counts {
		result[row.MusicID] = MusicEngagement{LikeCount: row.LikeCount}
	}
	if currentUserID == nil {
		return result, nil
	}
	var likedIDs []uint
	if err := r.db.WithContext(ctx).Model(&domain.UserMusicLike{}).
		Where("user_id = ? AND music_id IN ?", *currentUserID, musicIDs).
		Pluck("music_id", &likedIDs).Error; err != nil {
		return nil, err
	}
	for _, musicID := range likedIDs {
		engagement := result[musicID]
		engagement.IsLiked = true
		result[musicID] = engagement
	}
	return result, nil
}

func (r *musicRepository) ListLikedByUserID(ctx context.Context, userID uint, page, pageSize int) ([]*domain.Music, int64, error) {
	var musics []*domain.Music
	var total int64
	offset := (page - 1) * pageSize

	// Count total liked
	if err := r.db.WithContext(ctx).Model(&domain.UserMusicLike{}).Where("user_id = ?", userID).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// Join with vinyl table
	err := r.db.WithContext(ctx).Model(&domain.Music{}).
		Joins("JOIN user_music_likes ON user_music_likes.music_id = vinyl.id").
		Where("user_music_likes.user_id = ?", userID).
		Offset(offset).Limit(pageSize).
		Order("user_music_likes.created_at desc").
		Find(&musics).Error

	return musics, total, err
}
