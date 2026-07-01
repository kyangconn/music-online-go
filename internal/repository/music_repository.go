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
	Search(ctx context.Context, query string, page, pageSize int) ([]*domain.Music, int64, error)
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

func (r *musicRepository) Search(ctx context.Context, query string, page, pageSize int) ([]*domain.Music, int64, error) {
	var musics []*domain.Music
	var total int64
	offset := (page - 1) * pageSize

	db := r.db.WithContext(ctx).Model(&domain.Music{})
	if query != "" {
		likeQuery := "%" + query + "%"
		db = db.Where("title LIKE ? OR artist LIKE ?", likeQuery, likeQuery)
	}

	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	if err := db.Offset(offset).Limit(pageSize).Order("created_at desc").Find(&musics).Error; err != nil {
		return nil, 0, err
	}

	return musics, total, nil
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
	return r.db.WithContext(ctx).Delete(&domain.Music{}, id).Error
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
