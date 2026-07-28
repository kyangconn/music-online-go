package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/kyangconn/music-online-go/internal/domain"
	"gorm.io/gorm"
)

var (
	ErrArtistNotFound = errors.New("artist not found")
	ErrAlbumNotFound  = errors.New("album not found")
)

type BrowseRepository interface {
	ListArtists(ctx context.Context, params domain.BrowseArtistParams) ([]*domain.ArtistSummary, int64, error)
	FindArtist(ctx context.Context, key string) (*domain.ArtistSummary, error)
	ListAlbums(ctx context.Context, params domain.BrowseAlbumParams) ([]*domain.AlbumSummary, int64, error)
	FindAlbum(ctx context.Context, key string) (*domain.AlbumSummary, error)
}

type browseRepository struct {
	db *gorm.DB
}

func NewBrowseRepository(db *gorm.DB) BrowseRepository {
	return &browseRepository{db: db}
}

type artistAggregate struct {
	Key                 string
	Name                string
	MusicBrainzArtistID string
	TrackCount          int64
	AlbumCount          int64
	CoverMusicID        *uint
}

func (r *browseRepository) ListArtists(ctx context.Context, params domain.BrowseArtistParams) ([]*domain.ArtistSummary, int64, error) {
	base := r.artistQuery(ctx)
	if query := domain.NormalizeBrowseText(params.Query); query != "" {
		base = base.Where(`music_artist_credits.normalized_name LIKE ? ESCAPE '\'`, "%"+escapeLike(query)+"%")
	}

	var total int64
	if err := base.Distinct("music_artist_credits.group_key").Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count artists: %w", err)
	}

	var rows []artistAggregate
	offset := (params.Page - 1) * params.PageSize
	err := base.Select(`
		music_artist_credits.group_key AS key,
		MIN(music_artist_credits.name) AS name,
		MIN(music_artist_credits.music_brainz_artist_id) AS music_brainz_artist_id,
		COUNT(DISTINCT music_artist_credits.music_id) AS track_count,
		COUNT(DISTINCT music_album_memberships.group_key) AS album_count,
		MIN(CASE WHEN music_artist_credits.has_cover = ? THEN music_artist_credits.music_id ELSE NULL END) AS cover_music_id`, true).
		Group("music_artist_credits.group_key").
		Order("MIN(music_artist_credits.normalized_name) ASC, music_artist_credits.group_key ASC").
		Offset(offset).Limit(params.PageSize).Scan(&rows).Error
	if err != nil {
		return nil, 0, fmt.Errorf("list artists: %w", err)
	}
	return artistSummaries(rows), total, nil
}

func (r *browseRepository) FindArtist(ctx context.Context, key string) (*domain.ArtistSummary, error) {
	var row artistAggregate
	err := r.artistQuery(ctx).
		Where("music_artist_credits.group_key = ?", key).
		Select(`
			music_artist_credits.group_key AS key,
			MIN(music_artist_credits.name) AS name,
			MIN(music_artist_credits.music_brainz_artist_id) AS music_brainz_artist_id,
			COUNT(DISTINCT music_artist_credits.music_id) AS track_count,
			COUNT(DISTINCT music_album_memberships.group_key) AS album_count,
			MIN(CASE WHEN music_artist_credits.has_cover = ? THEN music_artist_credits.music_id ELSE NULL END) AS cover_music_id`, true).
		Group("music_artist_credits.group_key").Scan(&row).Error
	if err != nil {
		return nil, fmt.Errorf("find artist: %w", err)
	}
	if row.Key == "" {
		return nil, ErrArtistNotFound
	}
	return artistSummaries([]artistAggregate{row})[0], nil
}

func (r *browseRepository) artistQuery(ctx context.Context) *gorm.DB {
	return r.db.WithContext(ctx).Table("music_artist_credits").
		Joins("LEFT JOIN music_album_memberships ON music_album_memberships.music_id = music_artist_credits.music_id")
}

func artistSummaries(rows []artistAggregate) []*domain.ArtistSummary {
	result := make([]*domain.ArtistSummary, 0, len(rows))
	for _, row := range rows {
		result = append(result, &domain.ArtistSummary{
			Key: row.Key, Name: row.Name, MusicBrainzArtistID: row.MusicBrainzArtistID,
			TrackCount: row.TrackCount, AlbumCount: row.AlbumCount, CoverMusicID: row.CoverMusicID,
		})
	}
	return result
}

type albumAggregate struct {
	Key                       string
	Title                     string
	AlbumArtist               string
	AlbumArtistKey            string
	MusicBrainzReleaseID      string
	MusicBrainzReleaseGroupID string
	Year                      int
	TrackCount                int64
	TotalDuration             int64
	DiscCount                 int64
	CoverMusicID              *uint
}

func (r *browseRepository) ListAlbums(ctx context.Context, params domain.BrowseAlbumParams) ([]*domain.AlbumSummary, int64, error) {
	base := r.albumQuery(ctx, params)
	var total int64
	if err := base.Distinct("music_album_memberships.group_key").Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count albums: %w", err)
	}

	var rows []albumAggregate
	offset := (params.Page - 1) * params.PageSize
	err := selectAlbumAggregates(base).
		Order("MAX(music_album_memberships.year) DESC, MIN(music_album_memberships.normalized_title) ASC, music_album_memberships.group_key ASC").
		Offset(offset).Limit(params.PageSize).Scan(&rows).Error
	if err != nil {
		return nil, 0, fmt.Errorf("list albums: %w", err)
	}
	return albumSummaries(rows), total, nil
}

func (r *browseRepository) FindAlbum(ctx context.Context, key string) (*domain.AlbumSummary, error) {
	base := r.albumQuery(ctx, domain.BrowseAlbumParams{}).
		Where("music_album_memberships.group_key = ?", key)
	var row albumAggregate
	if err := selectAlbumAggregates(base).Scan(&row).Error; err != nil {
		return nil, fmt.Errorf("find album: %w", err)
	}
	if row.Key == "" {
		return nil, ErrAlbumNotFound
	}
	return albumSummaries([]albumAggregate{row})[0], nil
}

func (r *browseRepository) albumQuery(ctx context.Context, params domain.BrowseAlbumParams) *gorm.DB {
	db := r.db.WithContext(ctx).Table("music_album_memberships")
	if params.ArtistKey != "" {
		db = db.Joins("JOIN music_artist_credits ON music_artist_credits.music_id = music_album_memberships.music_id AND music_artist_credits.group_key = ?", params.ArtistKey)
	}
	if query := domain.NormalizeBrowseText(params.Query); query != "" {
		like := "%" + escapeLike(query) + "%"
		db = db.Where(`(music_album_memberships.normalized_title LIKE ? ESCAPE '\' OR music_album_memberships.normalized_album_artist LIKE ? ESCAPE '\')`, like, like)
	}
	if albumArtist := domain.NormalizeBrowseText(params.AlbumArtist); albumArtist != "" {
		db = db.Where("music_album_memberships.normalized_album_artist_key = ? AND music_album_memberships.normalized_album_artist = ?",
			domain.NormalizedBrowseTextKey(albumArtist), albumArtist)
	}
	if genre := domain.NormalizeGenreName(params.Genre); genre != "" {
		db = db.Joins("JOIN music_genre_facets ON music_genre_facets.music_id = music_album_memberships.music_id AND music_genre_facets.normalized_name = ?", genre)
	}
	if params.Year != nil {
		db = db.Where("music_album_memberships.year = ?", *params.Year)
	}
	return db
}

func selectAlbumAggregates(db *gorm.DB) *gorm.DB {
	return db.Select(`
		music_album_memberships.group_key AS key,
		MIN(music_album_memberships.title) AS title,
		MIN(music_album_memberships.album_artist) AS album_artist,
		MIN(music_album_memberships.album_artist_key) AS album_artist_key,
		MIN(music_album_memberships.music_brainz_release_id) AS music_brainz_release_id,
		MIN(music_album_memberships.music_brainz_release_group_id) AS music_brainz_release_group_id,
		MAX(music_album_memberships.year) AS year,
		COUNT(DISTINCT music_album_memberships.music_id) AS track_count,
		SUM(music_album_memberships.duration) AS total_duration,
		COUNT(DISTINCT CASE WHEN music_album_memberships.disc_number > 0 THEN music_album_memberships.disc_number ELSE 1 END) AS disc_count,
		MIN(CASE WHEN music_album_memberships.has_cover = ? THEN music_album_memberships.music_id ELSE NULL END) AS cover_music_id`, true).
		Group("music_album_memberships.group_key")
}

func albumSummaries(rows []albumAggregate) []*domain.AlbumSummary {
	result := make([]*domain.AlbumSummary, 0, len(rows))
	for _, row := range rows {
		result = append(result, &domain.AlbumSummary{
			Key: row.Key, Title: row.Title, AlbumArtist: row.AlbumArtist, AlbumArtistKey: row.AlbumArtistKey,
			MusicBrainzReleaseID:      row.MusicBrainzReleaseID,
			MusicBrainzReleaseGroupID: row.MusicBrainzReleaseGroupID,
			Year:                      row.Year, TrackCount: row.TrackCount, TotalDuration: row.TotalDuration,
			DiscCount: row.DiscCount, CoverMusicID: row.CoverMusicID,
		})
	}
	return result
}

// escapeLike keeps user text literal while retaining portable LIKE queries.
// Both browse queries explicitly declare backslash as the SQL LIKE escape.
func escapeLike(value string) string {
	value = strings.ReplaceAll(value, `\`, `\\`)
	value = strings.ReplaceAll(value, `%`, `\%`)
	return strings.ReplaceAll(value, `_`, `\_`)
}
