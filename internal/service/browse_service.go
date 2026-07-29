package service

import (
	"context"
	"errors"

	"github.com/kyangconn/music-online-go/internal/config"
	"github.com/kyangconn/music-online-go/internal/domain"
	"github.com/kyangconn/music-online-go/internal/repository"
)

var ErrInvalidBrowseKey = errors.New("invalid browse key")

type BrowseService interface {
	ListArtists(ctx context.Context, params domain.BrowseArtistParams) ([]*domain.ArtistSummary, int64, error)
	GetArtist(ctx context.Context, key string) (*domain.ArtistSummary, error)
	ListAlbums(ctx context.Context, params domain.BrowseAlbumParams) ([]*domain.AlbumSummary, int64, error)
	GetAlbum(ctx context.Context, key string) (*domain.AlbumSummary, error)
}

type browseService struct {
	repo      repository.BrowseRepository
	presenter musicPresenter
}

func NewBrowseService(repo repository.BrowseRepository, cfg config.Config) BrowseService {
	return &browseService{repo: repo, presenter: newMusicPresenter(cfg.Access, cfg.JWT)}
}

func (s *browseService) ListArtists(ctx context.Context, params domain.BrowseArtistParams) ([]*domain.ArtistSummary, int64, error) {
	artists, total, err := s.repo.ListArtists(ctx, params)
	if err != nil {
		return nil, 0, err
	}
	s.presentArtistCovers(artists)
	return artists, total, nil
}

func (s *browseService) GetArtist(ctx context.Context, key string) (*domain.ArtistSummary, error) {
	if !domain.IsBrowseGroupKey(key) {
		return nil, ErrInvalidBrowseKey
	}
	artist, err := s.repo.FindArtist(ctx, key)
	if err != nil {
		return nil, err
	}
	s.presentArtistCovers([]*domain.ArtistSummary{artist})
	return artist, nil
}

func (s *browseService) ListAlbums(ctx context.Context, params domain.BrowseAlbumParams) ([]*domain.AlbumSummary, int64, error) {
	if params.ArtistKey != "" && !domain.IsBrowseGroupKey(params.ArtistKey) {
		return nil, 0, ErrInvalidBrowseKey
	}
	albums, total, err := s.repo.ListAlbums(ctx, params)
	if err != nil {
		return nil, 0, err
	}
	s.presentAlbumCovers(albums)
	return albums, total, nil
}

func (s *browseService) GetAlbum(ctx context.Context, key string) (*domain.AlbumSummary, error) {
	if !domain.IsBrowseGroupKey(key) {
		return nil, ErrInvalidBrowseKey
	}
	album, err := s.repo.FindAlbum(ctx, key)
	if err != nil {
		return nil, err
	}
	s.presentAlbumCovers([]*domain.AlbumSummary{album})
	return album, nil
}

func (s *browseService) presentArtistCovers(artists []*domain.ArtistSummary) {
	for _, artist := range artists {
		if artist.CoverMusicID == nil {
			continue
		}
		artist.CoverURL, artist.CoverURLExpiresAt = s.presenter.cover(*artist.CoverMusicID)
	}
}

func (s *browseService) presentAlbumCovers(albums []*domain.AlbumSummary) {
	for _, album := range albums {
		if album.CoverMusicID == nil {
			continue
		}
		album.CoverURL, album.CoverURLExpiresAt = s.presenter.cover(*album.CoverMusicID)
	}
}
