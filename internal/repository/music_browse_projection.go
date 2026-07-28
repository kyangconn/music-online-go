package repository

import (
	"fmt"

	"github.com/kyangconn/music-online-go/internal/domain"
	"gorm.io/gorm"
)

// replaceMusicBrowseProjection keeps derived browse rows transactionally
// aligned with the canonical Music row. Callers must pass the transaction that
// created or updated the music record.
func replaceMusicBrowseProjection(tx *gorm.DB, music *domain.Music) error {
	if music == nil || music.ID == 0 {
		return fmt.Errorf("replace music browse projection: music ID is required")
	}
	if err := deleteMusicBrowseProjection(tx, []uint{music.ID}); err != nil {
		return err
	}

	projection := domain.BuildMusicBrowseProjection(music)
	if len(projection.ArtistCredits) > 0 {
		if err := tx.Create(&projection.ArtistCredits).Error; err != nil {
			return fmt.Errorf("create artist browse credits: %w", err)
		}
	}
	if projection.AlbumMembership != nil {
		if err := tx.Create(projection.AlbumMembership).Error; err != nil {
			return fmt.Errorf("create album browse membership: %w", err)
		}
	}
	if len(projection.GenreFacets) > 0 {
		if err := tx.Create(&projection.GenreFacets).Error; err != nil {
			return fmt.Errorf("create genre browse facets: %w", err)
		}
	}
	return nil
}

func deleteMusicBrowseProjection(tx *gorm.DB, musicIDs []uint) error {
	if len(musicIDs) == 0 {
		return nil
	}
	for _, model := range []any{
		&domain.MusicArtistCredit{},
		&domain.MusicAlbumMembership{},
		&domain.MusicGenreFacet{},
	} {
		if err := tx.Where("music_id IN ?", musicIDs).Delete(model).Error; err != nil {
			return fmt.Errorf("delete music browse projection: %w", err)
		}
	}
	return nil
}
