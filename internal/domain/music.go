// Package domain music.go - 音乐领域模型
// 定义 Music、UserMusicLike 实体及相关请求/响应 DTO
package domain

import (
	"fmt"
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

	Title        string     `json:"title" gorm:"size:255;not null;index"`
	Artist       string     `json:"artist" gorm:"size:255;not null;index"`
	Artists      StringList `json:"artists" gorm:"type:text"`
	Album        string     `json:"album" gorm:"size:255;index"`
	AlbumArtist  string     `json:"album_artist" gorm:"size:255;index"`
	AlbumArtists StringList `json:"album_artists" gorm:"type:text"`

	Year                int    `json:"year" gorm:"index"`
	TrackNumber         int    `json:"track_number"`
	TrackTotal          int    `json:"track_total"`
	DiscNumber          int    `json:"disc_number"`
	DiscTotal           int    `json:"disc_total"`
	ReleaseDate         string `json:"release_date" gorm:"size:10;index"`
	OriginalReleaseDate string `json:"original_release_date" gorm:"size:10;index"`

	Genre       string     `json:"genre" gorm:"size:500;index"`
	Genres      StringList `json:"genres" gorm:"type:text"`
	GenreTokens StringList `json:"genre_tokens" gorm:"type:text"`
	Comment     string     `json:"comment" gorm:"type:text"`
	ISRCs       StringList `json:"isrcs" gorm:"type:text"`
	Duration    int        `json:"duration"`
	Intro       string     `json:"intro" gorm:"type:text"`

	MusicBrainzRecordingID    string     `json:"musicbrainz_recording_id" gorm:"size:36;index"`
	MusicBrainzTrackID        string     `json:"musicbrainz_track_id" gorm:"size:36;index"`
	MusicBrainzReleaseID      string     `json:"musicbrainz_release_id" gorm:"size:36;index"`
	MusicBrainzReleaseGroupID string     `json:"musicbrainz_release_group_id" gorm:"size:36;index"`
	MusicBrainzArtistIDs      StringList `json:"musicbrainz_artist_ids" gorm:"type:text"`
	MusicBrainzAlbumArtistIDs StringList `json:"musicbrainz_album_artist_ids" gorm:"type:text"`
	MetadataRevision          uint64     `json:"metadata_revision" gorm:"not null;default:1"`

	Img         string    `json:"img" gorm:"size:500"`
	Path        string    `json:"path" gorm:"size:1000"`
	FileHash    string    `json:"file_hash" gorm:"size:64;index"`
	Type        MusicType `json:"type" gorm:"size:20;default:'single'"`
	IssuingDate time.Time `json:"issuing_date"`

	// Server-side library provenance. Root 0 is the managed upload
	// directory; positive IDs refer to administrator-registered read-only
	// roots. SourceKey is a hash of root + relative path and remains nullable
	// so legacy/browser-created records can coexist under a unique index.
	MediaRootID       uint       `json:"-" gorm:"not null;default:0;index"`
	MediaRelativePath string     `json:"-" gorm:"type:text"`
	MediaSourceKey    *string    `json:"-" gorm:"size:64;uniqueIndex"`
	SourceFileSize    int64      `json:"-" gorm:"not null;default:0"`
	SourceFileModTime *time.Time `json:"-"`
	// SourceReadOnly is a denormalized compatibility flag. MediaFile remains the
	// authority; true means at least one administrator-managed physical source
	// must not be mutated by an ordinary track owner.
	SourceReadOnly bool `json:"-" gorm:"not null;default:false"`

	// Uploader ID
	UserID uint `json:"user_id" gorm:"index"`

	// If it's a song in an album
	AlbumID *uint `json:"album_id" gorm:"index"`

	// Additional fields for response (not in DB table directly, populated via joins or separate queries)
	IsLiked   bool  `json:"is_liked" gorm:"-"`
	LikeCount int64 `json:"like_count" gorm:"-"`
}

// TableName overrides the table name used by User to `vinyl`
func (*Music) TableName() string {
	return "vinyl"
}

// UserMusicLike handles the many-to-many relationship for likes/collections
type UserMusicLike struct {
	UserID    uint      `gorm:"primaryKey"`
	MusicID   uint      `gorm:"primaryKey"`
	CreatedAt time.Time `json:"created_at"`
}

func (*UserMusicLike) TableName() string {
	return "user_music_likes"
}

// DTOs

type CreateMusicRequest struct {
	Title        string     `json:"title" binding:"required"`
	Artist       string     `json:"artist" binding:"required"`
	Artists      StringList `json:"artists"`
	Album        string     `json:"album"`
	AlbumArtist  string     `json:"album_artist"`
	AlbumArtists StringList `json:"album_artists"`

	Year                int    `json:"year" binding:"omitempty,min=1000,max=9999"`
	TrackNumber         int    `json:"track_number" binding:"omitempty,min=0"`
	TrackTotal          int    `json:"track_total" binding:"omitempty,min=0"`
	DiscNumber          int    `json:"disc_number" binding:"omitempty,min=0"`
	DiscTotal           int    `json:"disc_total" binding:"omitempty,min=0"`
	ReleaseDate         string `json:"release_date"`
	OriginalReleaseDate string `json:"original_release_date"`

	Genre    string     `json:"genre"`
	Genres   StringList `json:"genres"`
	Comment  string     `json:"comment"`
	ISRCs    StringList `json:"isrcs"`
	Duration int        `json:"duration" binding:"omitempty,min=0"`
	Intro    string     `json:"intro"`

	MusicBrainzRecordingID    string     `json:"musicbrainz_recording_id"`
	MusicBrainzTrackID        string     `json:"musicbrainz_track_id"`
	MusicBrainzReleaseID      string     `json:"musicbrainz_release_id"`
	MusicBrainzReleaseGroupID string     `json:"musicbrainz_release_group_id"`
	MusicBrainzArtistIDs      StringList `json:"musicbrainz_artist_ids"`
	MusicBrainzAlbumArtistIDs StringList `json:"musicbrainz_album_artist_ids"`

	Img         string    `json:"img"`
	Path        string    `json:"path"`
	Type        MusicType `json:"type" binding:"omitempty,oneof=single album"`
	IssuingDate time.Time `json:"issuing_date"`
	AlbumID     *uint     `json:"album_id"`
}

type UpdateMusicRequest struct {
	Title        *string     `json:"title"`
	Artist       *string     `json:"artist"`
	Artists      *StringList `json:"artists"`
	Album        *string     `json:"album"`
	AlbumArtist  *string     `json:"album_artist"`
	AlbumArtists *StringList `json:"album_artists"`

	Year                *int    `json:"year" binding:"omitempty,min=0,max=9999"`
	TrackNumber         *int    `json:"track_number" binding:"omitempty,min=0"`
	TrackTotal          *int    `json:"track_total" binding:"omitempty,min=0"`
	DiscNumber          *int    `json:"disc_number" binding:"omitempty,min=0"`
	DiscTotal           *int    `json:"disc_total" binding:"omitempty,min=0"`
	ReleaseDate         *string `json:"release_date"`
	OriginalReleaseDate *string `json:"original_release_date"`

	Genre    *string     `json:"genre"`
	Genres   *StringList `json:"genres"`
	Comment  *string     `json:"comment"`
	ISRCs    *StringList `json:"isrcs"`
	Duration *int        `json:"duration" binding:"omitempty,min=0"`
	Intro    *string     `json:"intro"`

	MusicBrainzRecordingID    *string     `json:"musicbrainz_recording_id"`
	MusicBrainzTrackID        *string     `json:"musicbrainz_track_id"`
	MusicBrainzReleaseID      *string     `json:"musicbrainz_release_id"`
	MusicBrainzReleaseGroupID *string     `json:"musicbrainz_release_group_id"`
	MusicBrainzArtistIDs      *StringList `json:"musicbrainz_artist_ids"`
	MusicBrainzAlbumArtistIDs *StringList `json:"musicbrainz_album_artist_ids"`

	Img         *string    `json:"img"`
	Path        *string    `json:"path"`
	Type        *MusicType `json:"type" binding:"omitempty,oneof=single album"`
	IssuingDate *time.Time `json:"issuing_date"`
	AlbumID     *uint      `json:"album_id"`
}

type MusicResponse struct {
	ID           uint       `json:"id"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
	Title        string     `json:"title"`
	Artist       string     `json:"artist"`
	Artists      StringList `json:"artists"`
	Album        string     `json:"album"`
	AlbumArtist  string     `json:"album_artist"`
	AlbumArtists StringList `json:"album_artists"`

	Year                int    `json:"year"`
	TrackNumber         int    `json:"track_number"`
	TrackTotal          int    `json:"track_total"`
	DiscNumber          int    `json:"disc_number"`
	DiscTotal           int    `json:"disc_total"`
	ReleaseDate         string `json:"release_date"`
	OriginalReleaseDate string `json:"original_release_date"`

	Genre       string     `json:"genre"`
	Genres      StringList `json:"genres"`
	GenreTokens StringList `json:"genre_tokens"`
	Comment     string     `json:"comment"`
	ISRCs       StringList `json:"isrcs"`
	Duration    int        `json:"duration"`
	Intro       string     `json:"intro"`

	MusicBrainzRecordingID    string                        `json:"musicbrainz_recording_id"`
	MusicBrainzTrackID        string                        `json:"musicbrainz_track_id"`
	MusicBrainzReleaseID      string                        `json:"musicbrainz_release_id"`
	MusicBrainzReleaseGroupID string                        `json:"musicbrainz_release_group_id"`
	MusicBrainzArtistIDs      StringList                    `json:"musicbrainz_artist_ids"`
	MusicBrainzAlbumArtistIDs StringList                    `json:"musicbrainz_album_artist_ids"`
	MetadataRevision          uint64                        `json:"metadata_revision"`
	ArtistKeys                StringList                    `json:"artist_keys"`
	AlbumKey                  string                        `json:"album_key,omitempty"`
	AlbumArtistKey            string                        `json:"album_artist_key,omitempty"`
	PresetClassification      *PresetClassificationResponse `json:"preset_classification,omitempty"`
	AudioAnalysis             *MusicAnalysisSummary         `json:"audio_analysis,omitempty"`

	Img               string     `json:"img"`
	Path              string     `json:"path"`
	Type              MusicType  `json:"type"`
	IssuingDate       time.Time  `json:"issuing_date"`
	UserID            uint       `json:"user_id"`
	SourceReadOnly    bool       `json:"source_read_only"`
	AlbumID           *uint      `json:"album_id"`
	IsLiked           bool       `json:"is_liked"`
	LikeCount         int64      `json:"like_count"`
	MediaURLExpiresAt *time.Time `json:"media_url_expires_at,omitempty"`
}

func (m *Music) ToResponse() *MusicResponse {
	if m == nil {
		return nil
	}
	resp := &MusicResponse{
		ID:                        m.ID,
		CreatedAt:                 m.CreatedAt,
		UpdatedAt:                 m.UpdatedAt,
		Title:                     m.Title,
		Artist:                    m.Artist,
		Artists:                   m.Artists,
		Album:                     m.Album,
		AlbumArtist:               m.AlbumArtist,
		AlbumArtists:              m.AlbumArtists,
		Year:                      m.Year,
		TrackNumber:               m.TrackNumber,
		TrackTotal:                m.TrackTotal,
		DiscNumber:                m.DiscNumber,
		DiscTotal:                 m.DiscTotal,
		ReleaseDate:               m.ReleaseDate,
		OriginalReleaseDate:       m.OriginalReleaseDate,
		Genre:                     m.Genre,
		Genres:                    m.Genres,
		GenreTokens:               m.GenreTokens,
		Comment:                   m.Comment,
		ISRCs:                     m.ISRCs,
		Duration:                  m.Duration,
		Intro:                     m.Intro,
		MusicBrainzRecordingID:    m.MusicBrainzRecordingID,
		MusicBrainzTrackID:        m.MusicBrainzTrackID,
		MusicBrainzReleaseID:      m.MusicBrainzReleaseID,
		MusicBrainzReleaseGroupID: m.MusicBrainzReleaseGroupID,
		MusicBrainzArtistIDs:      m.MusicBrainzArtistIDs,
		MusicBrainzAlbumArtistIDs: m.MusicBrainzAlbumArtistIDs,
		MetadataRevision:          m.MetadataRevision,
		ArtistKeys:                StringList{},
		Img:                       m.Img,
		Path:                      m.Path,
		Type:                      m.Type,
		IssuingDate:               m.IssuingDate,
		UserID:                    m.UserID,
		SourceReadOnly:            m.SourceReadOnly,
		AlbumID:                   m.AlbumID,
		IsLiked:                   m.IsLiked,
		LikeCount:                 m.LikeCount,
	}

	if resp.Path != "" {
		resp.Path = fmt.Sprintf("/api/v1/musics/%d/stream", m.ID)
	}
	if resp.Img != "" {
		resp.Img = fmt.Sprintf("/api/v1/musics/%d/cover", m.ID)
	}
	projection := BuildMusicBrowseProjection(m)
	for _, credit := range projection.ArtistCredits {
		if credit.TrackCredit {
			resp.ArtistKeys = append(resp.ArtistKeys, credit.GroupKey)
		}
	}
	if projection.AlbumMembership != nil {
		resp.AlbumKey = projection.AlbumMembership.GroupKey
		resp.AlbumArtistKey = projection.AlbumMembership.AlbumArtistKey
	}

	return resp
}

type MusicSearchParams struct {
	Query          string
	Title          string
	Artist         string
	ArtistKey      string
	Album          string
	AlbumKey       string
	AlbumArtist    string
	Genre          string
	Preset         string
	PresetStatus   string
	Year           *int
	MinYear        *int
	MaxYear        *int
	Duration       *int
	MinDuration    *int
	MaxDuration    *int
	RecordingID    string
	TrackID        string
	ReleaseID      string
	ReleaseGroupID string
	Type           *MusicType
	LikedOnly      bool
	LikedByUserID  *uint
	Page           int
	PageSize       int
	Offset         *int
}

type MusicFilterOptions struct {
	Artists      []string    `json:"artists"`
	Albums       []string    `json:"albums"`
	AlbumArtists []string    `json:"album_artists"`
	Genres       []string    `json:"genres"`
	Years        []int       `json:"years"`
	Types        []MusicType `json:"types"`
}

type MusicMetadata struct {
	Title        string     `json:"title"`
	Artist       string     `json:"artist"`
	Artists      StringList `json:"artists"`
	Album        string     `json:"album"`
	AlbumArtist  string     `json:"album_artist"`
	AlbumArtists StringList `json:"album_artists"`

	Year                int    `json:"year"`
	TrackNumber         int    `json:"track_number"`
	TrackTotal          int    `json:"track_total"`
	DiscNumber          int    `json:"disc_number"`
	DiscTotal           int    `json:"disc_total"`
	ReleaseDate         string `json:"release_date"`
	OriginalReleaseDate string `json:"original_release_date"`

	Genre    string     `json:"genre"`
	Genres   StringList `json:"genres"`
	Comment  string     `json:"comment"`
	ISRCs    StringList `json:"isrcs"`
	Duration int        `json:"duration"`

	MusicBrainzRecordingID    string     `json:"musicbrainz_recording_id"`
	MusicBrainzTrackID        string     `json:"musicbrainz_track_id"`
	MusicBrainzReleaseID      string     `json:"musicbrainz_release_id"`
	MusicBrainzReleaseGroupID string     `json:"musicbrainz_release_group_id"`
	MusicBrainzArtistIDs      StringList `json:"musicbrainz_artist_ids"`
	MusicBrainzAlbumArtistIDs StringList `json:"musicbrainz_album_artist_ids"`
}

type MusicDuplicateCheckRequest struct {
	FileHash string `json:"file_hash" binding:"omitempty,len=64,hexadecimal"`
	MusicMetadata
}

func (r *MusicDuplicateCheckRequest) Metadata() MusicMetadata {
	return MusicMetadata{
		Title:                     r.Title,
		Artist:                    r.Artist,
		Artists:                   r.Artists,
		Album:                     r.Album,
		AlbumArtist:               r.AlbumArtist,
		AlbumArtists:              r.AlbumArtists,
		Year:                      r.Year,
		TrackNumber:               r.TrackNumber,
		TrackTotal:                r.TrackTotal,
		DiscNumber:                r.DiscNumber,
		DiscTotal:                 r.DiscTotal,
		ReleaseDate:               r.ReleaseDate,
		OriginalReleaseDate:       r.OriginalReleaseDate,
		Genre:                     r.Genre,
		Genres:                    r.Genres,
		Comment:                   r.Comment,
		ISRCs:                     r.ISRCs,
		Duration:                  r.Duration,
		MusicBrainzRecordingID:    r.MusicBrainzRecordingID,
		MusicBrainzTrackID:        r.MusicBrainzTrackID,
		MusicBrainzReleaseID:      r.MusicBrainzReleaseID,
		MusicBrainzReleaseGroupID: r.MusicBrainzReleaseGroupID,
		MusicBrainzArtistIDs:      r.MusicBrainzArtistIDs,
		MusicBrainzAlbumArtistIDs: r.MusicBrainzAlbumArtistIDs,
	}
}

type MusicDuplicateCheckResponse struct {
	ExactMatch        *MusicResponse      `json:"exact_match,omitempty"`
	MetadataMatches   []*MusicResponse    `json:"metadata_matches"`
	SuggestedMetadata MusicMetadata       `json:"suggested_metadata"`
	Enrichment        *UpdateMusicRequest `json:"enrichment,omitempty"`
}
