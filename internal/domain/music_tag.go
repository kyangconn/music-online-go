package domain

import (
	"time"

	"gorm.io/gorm"
)

type MusicTag struct {
	ID        uint           `json:"id" gorm:"primarykey"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `json:"-" gorm:"index"`

	// Core fields (at least artist + title required)
	Artist    string    `json:"artist" gorm:"size:255;index:idx_artist_title;index:idx_artist"`
	Title     string    `json:"title" gorm:"size:255;index:idx_artist_title;index:idx_title"`
	Album     string    `json:"album" gorm:"size:255;index:idx_album"`
	AlbumArtist string   `json:"album_artist" gorm:"size:255;index:idx_album_artist"`
	TrackNumber int     `json:"track_number" gorm:"index:idx_track_number"`
	DiscNumber  int     `json:"disc_number" gorm:"index:idx_disc_number"`
	
	// Additional metadata
	Genre     string    `json:"genre" gorm:"size:100;index:idx_genre"`
	Year      int       `json:"year" gorm:"index:idx_year"`
	Duration  int       `json:"duration" gorm:"index:idx_duration"` // in seconds
	Comment   string    `json:"comment" gorm:"type:text"`
	
	// External IDs (for MusicBrainz compatibility)
	MusicBrainzID  string `json:"musicbrainz_id" gorm:"size:36;index:idx_musicbrainz_id"`
	MusicBrainzArtistID string `json:"musicbrainz_artist_id" gorm:"size:36;index:idx_musicbrainz_artist_id"`
	
	// Statistics
	UseCount int `json:"use_count" gorm:"default:0"` // How many music records use this tag
	
	// Index for fuzzy search
	SearchVector string `json:"search_vector" gorm:"type:tsvector"`
}

func (MusicTag) TableName() string {
	return "music_tags"
}

// DTOs

type CreateMusicTagRequest struct {
	Artist          string `json:"artist" binding:"required"`
	Title           string `json:"title" binding:"required"`
	Album           string `json:"album"`
	AlbumArtist     string `json:"album_artist"`
	TrackNumber     *int   `json:"track_number"`
	DiscNumber      *int   `json:"disc_number"`
	Genre           string `json:"genre"`
	Year            *int   `json:"year"`
	Duration        *int   `json:"duration"`
	Comment         string `json:"comment"`
	MusicBrainzID   string `json:"musicbrainz_id"`
	MusicBrainzArtistID string `json:"musicbrainz_artist_id"`
}

type UpdateMusicTagRequest struct {
	Artist          *string `json:"artist"`
	Title           *string `json:"title"`
	Album           *string `json:"album"`
	AlbumArtist     *string `json:"album_artist"`
	TrackNumber     *int    `json:"track_number"`
	DiscNumber      *int    `json:"disc_number"`
	Genre           *string `json:"genre"`
	Year            *int    `json:"year"`
	Duration        *int    `json:"duration"`
	Comment         *string `json:"comment"`
	MusicBrainzID   *string `json:"musicbrainz_id"`
	MusicBrainzArtistID *string `json:"musicbrainz_artist_id"`
}

type MusicTagResponse struct {
	ID                  uint      `json:"id"`
	CreatedAt           time.Time `json:"created_at"`
	UpdatedAt           time.Time `json:"updated_at"`
	Artist              string    `json:"artist"`
	Title               string    `json:"title"`
	Album               string    `json:"album"`
	AlbumArtist         string    `json:"album_artist"`
	TrackNumber         int       `json:"track_number"`
	DiscNumber          int       `json:"disc_number"`
	Genre               string    `json:"genre"`
	Year                int       `json:"year"`
	Duration            int       `json:"duration"`
	Comment             string    `json:"comment"`
	MusicBrainzID       string    `json:"musicbrainz_id"`
	MusicBrainzArtistID string    `json:"musicbrainz_artist_id"`
	UseCount            int       `json:"use_count"`
}

func (mt *MusicTag) ToResponse() *MusicTagResponse {
	if mt == nil {
		return nil
	}
	return &MusicTagResponse{
		ID:                  mt.ID,
		CreatedAt:           mt.CreatedAt,
		UpdatedAt:           mt.UpdatedAt,
		Artist:              mt.Artist,
		Title:               mt.Title,
		Album:               mt.Album,
		AlbumArtist:         mt.AlbumArtist,
		TrackNumber:         mt.TrackNumber,
		DiscNumber:          mt.DiscNumber,
		Genre:               mt.Genre,
		Year:                mt.Year,
		Duration:            mt.Duration,
		Comment:             mt.Comment,
		MusicBrainzID:       mt.MusicBrainzID,
		MusicBrainzArtistID: mt.MusicBrainzArtistID,
		UseCount:            mt.UseCount,
	}
}

// SearchParams for tag search
type TagSearchParams struct {
	Artist        string `form:"artist" json:"artist"`
	Title         string `form:"title" json:"title"`
	Album         string `form:"album" json:"album"`
	AlbumArtist   string `form:"album_artist" json:"album_artist"`
	Genre         string `form:"genre" json:"genre"`
	Year          *int   `form:"year" json:"year"`
	MinYear       *int   `form:"min_year" json:"min_year"`
	MaxYear       *int   `form:"max_year" json:"max_year"`
	Duration      *int   `form:"duration" json:"duration"`
	MinDuration   *int   `form:"min_duration" json:"min_duration"`
	MaxDuration   *int   `form:"max_duration" json:"max_duration"`
	MusicBrainzID string `form:"musicbrainz_id" json:"musicbrainz_id"`
	Limit         int    `form:"limit" json:"limit"`
	Offset        int    `form:"offset" json:"offset"`
}

func (s *TagSearchParams) GetLimit() int {
	if s.Limit <= 0 || s.Limit > 100 {
		return 20
	}
	return s.Limit
}

func (s *TagSearchParams) GetOffset() int {
	if s.Offset < 0 {
		return 0
	}
	return s.Offset
}
