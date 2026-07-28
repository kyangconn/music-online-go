package domain

import "time"

const MaxPlaylistItems = 500

type Playlist struct {
	ID          uint      `json:"id" gorm:"primarykey"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	UserID      uint      `json:"user_id" gorm:"not null;index"`
	Name        string    `json:"name" gorm:"size:120;not null"`
	Description string    `json:"description" gorm:"size:1000"`
	Revision    uint64    `json:"revision" gorm:"not null;default:1"`
	ItemCount   int64     `json:"item_count" gorm:"-"`
	User        User      `json:"-" gorm:"foreignKey:UserID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
}

func (*Playlist) TableName() string {
	return "playlists"
}

// PlaylistItem is unique per track in a playlist. Position is intentionally
// not a unique constraint: playlist-row transactions serialize mutations, and
// the music ID is a deterministic tie-breaker after interrupted legacy writes.
type PlaylistItem struct {
	PlaylistID uint      `json:"playlist_id" gorm:"primaryKey;index:idx_playlist_item_order,priority:1"`
	MusicID    uint      `json:"music_id" gorm:"primaryKey;index"`
	Position   int       `json:"position" gorm:"not null;index:idx_playlist_item_order,priority:2"`
	CreatedAt  time.Time `json:"created_at"`
	Playlist   Playlist  `json:"-" gorm:"foreignKey:PlaylistID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
	Music      Music     `json:"-" gorm:"foreignKey:MusicID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
}

func (*PlaylistItem) TableName() string {
	return "playlist_items"
}

type CreatePlaylistRequest struct {
	Name        string `json:"name" binding:"required"`
	Description string `json:"description"`
}

type UpdatePlaylistRequest struct {
	Name        *string `json:"name"`
	Description *string `json:"description"`
}

type AddPlaylistItemRequest struct {
	MusicID uint `json:"music_id" binding:"required"`
}

type ReorderPlaylistItemsRequest struct {
	MusicIDs []uint `json:"music_ids" binding:"required"`
}

type PlaylistItemResponse struct {
	Position int            `json:"position"`
	Music    *MusicResponse `json:"music"`
}

type PlaylistDetailResponse struct {
	Playlist
	Items []PlaylistItemResponse `json:"items"`
}
