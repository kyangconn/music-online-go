package domain

import (
	"time"

	"gorm.io/gorm"
)

type MusicType string

const (
	MusicTypeSingle MusicType = "single"
	MusicTypeAlbum  MusicType = "album"
)

type Music struct {
	ID        uint           `json:"id" gorm:"primarykey"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `json:"-" gorm:"index"`

	Title       string    `json:"title" gorm:"size:255;not null;index"`
	Artist      string    `json:"artist" gorm:"size:255;not null;index"`
	Intro       string    `json:"intro" gorm:"type:text"`
	Img         string    `json:"img" gorm:"size:500"`
	Path        string    `json:"path" gorm:"size:500"`
	Type        MusicType `json:"type" gorm:"size:20;default:'single'"`
	IssuingDate time.Time `json:"issuing_date"`

	// Uploader ID
	UserID uint `json:"user_id" gorm:"index"`

	// If it's a song in an album
	AlbumID *uint `json:"album_id" gorm:"index"`

	// Additional fields for response (not in DB table directly, populated via joins or separate queries)
	IsLiked   bool  `json:"is_liked" gorm:"-"`
	LikeCount int64 `json:"like_count" gorm:"-"`
}

// TableName overrides the table name used by User to `vinyl`
func (Music) TableName() string {
	return "vinyl"
}

// UserMusicLike handles the many-to-many relationship for likes/collections
type UserMusicLike struct {
	UserID    uint      `gorm:"primaryKey"`
	MusicID   uint      `gorm:"primaryKey"`
	CreatedAt time.Time `json:"created_at"`
}

func (UserMusicLike) TableName() string {
	return "user_music_likes"
}

// DTOs

type CreateMusicRequest struct {
	Title       string    `json:"title" binding:"required"`
	Artist      string    `json:"artist" binding:"required"`
	Intro       string    `json:"intro"`
	Img         string    `json:"img"`
	Path        string    `json:"path"`
	Type        MusicType `json:"type" binding:"oneof=single album"`
	IssuingDate time.Time `json:"issuing_date"`
	AlbumID     *uint     `json:"album_id"`
}

type UpdateMusicRequest struct {
	Title       *string    `json:"title"`
	Artist      *string    `json:"artist"`
	Intro       *string    `json:"intro"`
	Img         *string    `json:"img"`
	Path        *string    `json:"path"`
	Type        *MusicType `json:"type" binding:"omitempty,oneof=single album"`
	IssuingDate *time.Time `json:"issuing_date"`
	AlbumID     *uint      `json:"album_id"`
}

type MusicResponse struct {
	ID          uint      `json:"id"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	Title       string    `json:"title"`
	Artist      string    `json:"artist"`
	Intro       string    `json:"intro"`
	Img         string    `json:"img"`
	Path        string    `json:"path"`
	Type        MusicType `json:"type"`
	IssuingDate time.Time `json:"issuing_date"`
	UserID      uint      `json:"user_id"`
	AlbumID     *uint     `json:"album_id"`
	IsLiked     bool      `json:"is_liked"`
	LikeCount   int64     `json:"like_count"`
}

func (m *Music) ToResponse() *MusicResponse {
	if m == nil {
		return nil
	}
	return &MusicResponse{
		ID:          m.ID,
		CreatedAt:   m.CreatedAt,
		UpdatedAt:   m.UpdatedAt,
		Title:       m.Title,
		Artist:      m.Artist,
		Intro:       m.Intro,
		Img:         m.Img,
		Type:        m.Type,
		IssuingDate: m.IssuingDate,
		UserID:      m.UserID,
		AlbumID:     m.AlbumID,
		IsLiked:     m.IsLiked,
		LikeCount:   m.LikeCount,
	}
}
