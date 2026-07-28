package repository

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/kyangconn/music-online-go/internal/domain"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	ErrPlaylistNotFound      = errors.New("playlist not found")
	ErrPlaylistItemNotFound  = errors.New("playlist item not found")
	ErrPlaylistItemsMismatch = errors.New("playlist reorder must contain every current music ID exactly once")
	ErrPlaylistFull          = errors.New("playlist item limit reached")
)

type PlaylistRepository interface {
	Create(ctx context.Context, playlist *domain.Playlist) error
	ListByUserID(ctx context.Context, userID uint, page, pageSize int) ([]*domain.Playlist, int64, error)
	FindOwnedByID(ctx context.Context, id, userID uint) (*domain.Playlist, error)
	ListItems(ctx context.Context, id, userID uint) ([]*domain.PlaylistItem, error)
	Update(ctx context.Context, id, userID uint, name, description *string) error
	Delete(ctx context.Context, id, userID uint) error
	AddItem(ctx context.Context, id, userID, musicID uint) error
	RemoveItem(ctx context.Context, id, userID, musicID uint) error
	ReorderItems(ctx context.Context, id, userID uint, musicIDs []uint) error
}

type playlistRepository struct {
	db *gorm.DB
}

func NewPlaylistRepository(db *gorm.DB) PlaylistRepository {
	return &playlistRepository{db: db}
}

func (r *playlistRepository) Create(ctx context.Context, playlist *domain.Playlist) error {
	return r.db.WithContext(ctx).Create(playlist).Error
}

func (r *playlistRepository) ListByUserID(ctx context.Context, userID uint, page, pageSize int) ([]*domain.Playlist, int64, error) {
	var total int64
	base := r.db.WithContext(ctx).Model(&domain.Playlist{}).Where("user_id = ?", userID)
	if err := base.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count playlists: %w", err)
	}
	var playlists []*domain.Playlist
	err := base.Select(`playlists.*, (
		SELECT COUNT(*) FROM playlist_items WHERE playlist_items.playlist_id = playlists.id
	) AS item_count`).Order("updated_at DESC, id DESC").
		Offset((page - 1) * pageSize).Limit(pageSize).Find(&playlists).Error
	if err != nil {
		return nil, 0, fmt.Errorf("list playlists: %w", err)
	}
	return playlists, total, nil
}

func (r *playlistRepository) FindOwnedByID(ctx context.Context, id, userID uint) (*domain.Playlist, error) {
	var playlist domain.Playlist
	err := r.db.WithContext(ctx).Model(&domain.Playlist{}).
		Select(`playlists.*, (
			SELECT COUNT(*) FROM playlist_items WHERE playlist_items.playlist_id = playlists.id
		) AS item_count`).Where("id = ? AND user_id = ?", id, userID).First(&playlist).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrPlaylistNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("find playlist: %w", err)
	}
	return &playlist, nil
}

func (r *playlistRepository) ListItems(ctx context.Context, id, userID uint) ([]*domain.PlaylistItem, error) {
	if _, err := r.FindOwnedByID(ctx, id, userID); err != nil {
		return nil, err
	}
	var items []*domain.PlaylistItem
	if err := r.db.WithContext(ctx).Where("playlist_id = ?", id).
		Order("position ASC, music_id ASC").Find(&items).Error; err != nil {
		return nil, fmt.Errorf("list playlist items: %w", err)
	}
	return items, nil
}

func (r *playlistRepository) Update(ctx context.Context, id, userID uint, name, description *string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := ensureOwnedPlaylist(tx, id, userID); err != nil {
			return err
		}
		updates := map[string]any{"revision": gorm.Expr("revision + 1")}
		if name != nil {
			updates["name"] = *name
		}
		if description != nil {
			updates["description"] = *description
		}
		return tx.Model(&domain.Playlist{}).Where("id = ?", id).Updates(updates).Error
	})
}

func (r *playlistRepository) Delete(ctx context.Context, id, userID uint) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := ensureOwnedPlaylist(tx, id, userID); err != nil {
			return err
		}
		if err := tx.Where("playlist_id = ?", id).Delete(&domain.PlaylistItem{}).Error; err != nil {
			return err
		}
		return tx.Delete(&domain.Playlist{}, id).Error
	})
}

func (r *playlistRepository) AddItem(ctx context.Context, id, userID, musicID uint) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := ensureOwnedPlaylist(tx, id, userID); err != nil {
			return err
		}
		var musicCount int64
		if err := tx.Model(&domain.Music{}).Where("id = ?", musicID).Count(&musicCount).Error; err != nil {
			return err
		}
		if musicCount == 0 {
			return ErrMusicNotFound
		}
		var existing int64
		if err := tx.Model(&domain.PlaylistItem{}).
			Where("playlist_id = ? AND music_id = ?", id, musicID).Count(&existing).Error; err != nil {
			return err
		}
		if existing > 0 {
			return nil
		}
		var itemCount int64
		if err := tx.Model(&domain.PlaylistItem{}).Where("playlist_id = ?", id).Count(&itemCount).Error; err != nil {
			return err
		}
		if itemCount >= domain.MaxPlaylistItems {
			return ErrPlaylistFull
		}
		var lastPosition int
		if err := tx.Model(&domain.PlaylistItem{}).Select("COALESCE(MAX(position), -1)").
			Where("playlist_id = ?", id).Scan(&lastPosition).Error; err != nil {
			return err
		}
		item := &domain.PlaylistItem{PlaylistID: id, MusicID: musicID, Position: lastPosition + 1}
		if err := tx.Create(item).Error; err != nil {
			return err
		}
		return touchPlaylist(tx, id)
	})
}

func (r *playlistRepository) RemoveItem(ctx context.Context, id, userID, musicID uint) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := ensureOwnedPlaylist(tx, id, userID); err != nil {
			return err
		}
		result := tx.Where("playlist_id = ? AND music_id = ?", id, musicID).Delete(&domain.PlaylistItem{})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return ErrPlaylistItemNotFound
		}
		if err := compactPlaylistPositions(tx, id); err != nil {
			return err
		}
		return touchPlaylist(tx, id)
	})
}

func (r *playlistRepository) ReorderItems(ctx context.Context, id, userID uint, musicIDs []uint) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := ensureOwnedPlaylist(tx, id, userID); err != nil {
			return err
		}
		var current []uint
		if err := tx.Model(&domain.PlaylistItem{}).Where("playlist_id = ?", id).
			Order("position ASC, music_id ASC").Pluck("music_id", &current).Error; err != nil {
			return err
		}
		if !sameUniqueIDs(current, musicIDs) {
			return ErrPlaylistItemsMismatch
		}
		if slices.Equal(current, musicIDs) {
			return nil
		}
		for position, musicID := range musicIDs {
			if err := tx.Model(&domain.PlaylistItem{}).
				Where("playlist_id = ? AND music_id = ?", id, musicID).
				Update("position", position).Error; err != nil {
				return err
			}
		}
		return touchPlaylist(tx, id)
	})
}

func ensureOwnedPlaylist(tx *gorm.DB, id, userID uint) error {
	query := tx.Select("id").Where("id = ? AND user_id = ?", id, userID)
	// PostgreSQL needs an explicit row lock so concurrent instances cannot
	// assign positions from the same snapshot. SQLite deployments already use a
	// single writer connection and do not support SELECT ... FOR UPDATE.
	if tx.Dialector.Name() == "postgres" {
		query = query.Clauses(clause.Locking{Strength: "UPDATE"})
	}
	var playlist domain.Playlist
	if err := query.First(&playlist).Error; errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrPlaylistNotFound
	} else if err != nil {
		return err
	}
	return nil
}

func touchPlaylist(tx *gorm.DB, id uint) error {
	return tx.Model(&domain.Playlist{}).Where("id = ?", id).
		Updates(map[string]any{"revision": gorm.Expr("revision + 1"), "updated_at": gorm.Expr("CURRENT_TIMESTAMP")}).Error
}

func compactPlaylistPositions(tx *gorm.DB, id uint) error {
	var items []domain.PlaylistItem
	if err := tx.Where("playlist_id = ?", id).Order("position ASC, music_id ASC").Find(&items).Error; err != nil {
		return err
	}
	for position, item := range items {
		if item.Position == position {
			continue
		}
		if err := tx.Model(&domain.PlaylistItem{}).
			Where("playlist_id = ? AND music_id = ?", id, item.MusicID).
			Update("position", position).Error; err != nil {
			return err
		}
	}
	return nil
}

func sameUniqueIDs(current, requested []uint) bool {
	if len(current) != len(requested) {
		return false
	}
	seen := make(map[uint]struct{}, len(requested))
	for _, id := range current {
		seen[id] = struct{}{}
	}
	for _, id := range requested {
		if _, exists := seen[id]; !exists {
			return false
		}
		delete(seen, id)
	}
	return len(seen) == 0
}

func removeMusicFromPlaylists(tx *gorm.DB, musicIDs []uint) error {
	if len(musicIDs) == 0 {
		return nil
	}
	var playlistIDs []uint
	if err := tx.Model(&domain.PlaylistItem{}).Where("music_id IN ?", musicIDs).
		Distinct("playlist_id").Pluck("playlist_id", &playlistIDs).Error; err != nil {
		return err
	}
	if len(playlistIDs) == 0 {
		return nil
	}
	if err := tx.Where("music_id IN ?", musicIDs).Delete(&domain.PlaylistItem{}).Error; err != nil {
		return err
	}
	for _, playlistID := range playlistIDs {
		if err := compactPlaylistPositions(tx, playlistID); err != nil {
			return err
		}
		if err := touchPlaylist(tx, playlistID); err != nil {
			return err
		}
	}
	return nil
}
