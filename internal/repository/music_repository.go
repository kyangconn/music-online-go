package repository

import (
	"errors"

	"github.com/kyangconn/music-online-web/internal/domain"
	"gorm.io/gorm"
)

var (
	ErrMusicNotFound = errors.New("music not found")
)

type MusicRepository interface {
	Create(music *domain.Music) error
	FindByID(id uint) (*domain.Music, error)
	Search(query string, page, pageSize int) ([]*domain.Music, int64, error)
	ListByUserID(userID uint, page, pageSize int) ([]*domain.Music, int64, error)
	Update(music *domain.Music) error
	Delete(id uint) error

	// Like/Collection
	LikeMusic(userID, musicID uint) error
	UnlikeMusic(userID, musicID uint) error
	IsLiked(userID, musicID uint) (bool, error)
	CountLikes(musicID uint) (int64, error)
	ListLikedByUserID(userID uint, page, pageSize int) ([]*domain.Music, int64, error)
}

type musicRepository struct {
	db *gorm.DB
}

func NewMusicRepository(db *gorm.DB) MusicRepository {
	return &musicRepository{db: db}
}

func (r *musicRepository) Create(music *domain.Music) error {
	return r.db.Create(music).Error
}

func (r *musicRepository) FindByID(id uint) (*domain.Music, error) {
	var music domain.Music
	if err := r.db.First(&music, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrMusicNotFound
		}
		return nil, err
	}
	return &music, nil
}

func (r *musicRepository) Search(query string, page, pageSize int) ([]*domain.Music, int64, error) {
	var musics []*domain.Music
	var total int64
	offset := (page - 1) * pageSize

	db := r.db.Model(&domain.Music{})
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

func (r *musicRepository) ListByUserID(userID uint, page, pageSize int) ([]*domain.Music, int64, error) {
	var musics []*domain.Music
	var total int64
	offset := (page - 1) * pageSize

	db := r.db.Model(&domain.Music{}).Where("user_id = ?", userID)

	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	if err := db.Offset(offset).Limit(pageSize).Order("created_at desc").Find(&musics).Error; err != nil {
		return nil, 0, err
	}

	return musics, total, nil
}

func (r *musicRepository) Update(music *domain.Music) error {
	return r.db.Save(music).Error
}

func (r *musicRepository) Delete(id uint) error {
	return r.db.Delete(&domain.Music{}, id).Error
}

func (r *musicRepository) LikeMusic(userID, musicID uint) error {
	like := domain.UserMusicLike{
		UserID:  userID,
		MusicID: musicID,
	}
	// FirstOrCreate ensures idempotency
	return r.db.FirstOrCreate(&like, like).Error
}

func (r *musicRepository) UnlikeMusic(userID, musicID uint) error {
	return r.db.Where("user_id = ? AND music_id = ?", userID, musicID).Delete(&domain.UserMusicLike{}).Error
}

func (r *musicRepository) IsLiked(userID, musicID uint) (bool, error) {
	var count int64
	err := r.db.Model(&domain.UserMusicLike{}).
		Where("user_id = ? AND music_id = ?", userID, musicID).
		Count(&count).Error
	return count > 0, err
}

func (r *musicRepository) CountLikes(musicID uint) (int64, error) {
	var count int64
	err := r.db.Model(&domain.UserMusicLike{}).
		Where("music_id = ?", musicID).
		Count(&count).Error
	return count, err
}

func (r *musicRepository) ListLikedByUserID(userID uint, page, pageSize int) ([]*domain.Music, int64, error) {
	var musics []*domain.Music
	var total int64
	offset := (page - 1) * pageSize

	// Count total liked
	if err := r.db.Model(&domain.UserMusicLike{}).Where("user_id = ?", userID).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// Join with vinyl table
	err := r.db.Model(&domain.Music{}).
		Joins("JOIN user_music_likes ON user_music_likes.music_id = vinyl.id").
		Where("user_music_likes.user_id = ?", userID).
		Offset(offset).Limit(pageSize).
		Order("user_music_likes.created_at desc").
		Find(&musics).Error

	return musics, total, err
}
