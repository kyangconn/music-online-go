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
	ListLikedByUserID(ctx context.Context, userID uint, page, pageSize int) ([]*domain.Music, int64, error)
}

type musicRepository struct {
	db *gorm.DB
}

func NewMusicRepository(db *gorm.DB) MusicRepository {
	return &musicRepository{db: db}
}

func (r *musicRepository) Create(ctx context.Context, music *domain.Music) error {
	return r.db.WithContext(ctx).Create(music).Error
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
	if params.Query != "" {
		likeQuery := "%" + params.Query + "%"
		db = db.Where("LOWER(title) LIKE LOWER(?) OR LOWER(artist) LIKE LOWER(?)", likeQuery, likeQuery)
	}
	if params.Title != "" {
		db = db.Where("LOWER(title) = LOWER(?)", params.Title)
	}
	if params.Artist != "" {
		db = db.Where("LOWER(artist) = LOWER(?)", params.Artist)
	}
	if params.Album != "" {
		db = db.Where("LOWER(album) = LOWER(?)", params.Album)
	}
	if params.AlbumArtist != "" {
		db = db.Where("LOWER(album_artist) = LOWER(?)", params.AlbumArtist)
	}
	if params.Genre != "" {
		genre := "%" + params.Genre + "%"
		db = db.Where("LOWER(genre) LIKE LOWER(?)", genre)
	}
	if params.Year != nil {
		db = db.Where("year = ?", *params.Year)
	}
	if params.MinYear != nil {
		db = db.Where("year >= ?", *params.MinYear)
	}
	if params.MaxYear != nil {
		db = db.Where("year <= ?", *params.MaxYear)
	}
	if params.Duration != nil {
		db = db.Where("duration = ?", *params.Duration)
	}
	if params.MinDuration != nil {
		db = db.Where("duration >= ?", *params.MinDuration)
	}
	if params.MaxDuration != nil {
		db = db.Where("duration <= ?", *params.MaxDuration)
	}
	if params.RecordingID != "" {
		db = db.Where("music_brainz_recording_id = ?", params.RecordingID)
	}
	if params.TrackID != "" {
		db = db.Where("music_brainz_track_id = ?", params.TrackID)
	}
	if params.ReleaseID != "" {
		db = db.Where("music_brainz_release_id = ?", params.ReleaseID)
	}
	if params.ReleaseGroupID != "" {
		db = db.Where("music_brainz_release_group_id = ?", params.ReleaseGroupID)
	}
	if params.Type != nil {
		db = db.Where("type = ?", *params.Type)
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

	if err := db.Offset(offset).Limit(params.PageSize).Order("vinyl.created_at DESC").Find(&musics).Error; err != nil {
		return nil, 0, err
	}

	return musics, total, nil
}

func (r *musicRepository) ListFilterOptions(ctx context.Context) (*domain.MusicFilterOptions, error) {
	options := &domain.MusicFilterOptions{
		Artists: []string{},
		Years:   []int{},
		Types:   []domain.MusicType{},
	}

	if err := r.db.WithContext(ctx).Model(&domain.Music{}).
		Where("artist <> ''").Distinct("artist").Order("artist ASC").Pluck("artist", &options.Artists).Error; err != nil {
		return nil, err
	}
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
	return r.db.WithContext(ctx).Save(music).Error
}

func (r *musicRepository) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
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
