package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/kyangconn/music-online-go/internal/domain"
	"github.com/kyangconn/music-online-go/internal/repository"
)

var ErrInvalidPlaylist = errors.New("invalid playlist")

const (
	maxPlaylistNameCharacters        = 120
	maxPlaylistDescriptionCharacters = 1000
)

type PlaylistService interface {
	Create(ctx context.Context, userID uint, req *domain.CreatePlaylistRequest) (*domain.PlaylistDetailResponse, error)
	List(ctx context.Context, userID uint, page, pageSize int) ([]*domain.Playlist, int64, error)
	Get(ctx context.Context, userID, id uint) (*domain.PlaylistDetailResponse, error)
	Update(ctx context.Context, userID, id uint, req *domain.UpdatePlaylistRequest) (*domain.PlaylistDetailResponse, error)
	Delete(ctx context.Context, userID, id uint) error
	AddItem(ctx context.Context, userID, id, musicID uint) (*domain.PlaylistDetailResponse, error)
	RemoveItem(ctx context.Context, userID, id, musicID uint) (*domain.PlaylistDetailResponse, error)
	ReorderItems(ctx context.Context, userID, id uint, musicIDs []uint) (*domain.PlaylistDetailResponse, error)
}

type playlistService struct {
	repo         repository.PlaylistRepository
	musicService MusicService
}

func NewPlaylistService(repo repository.PlaylistRepository, musicService MusicService) PlaylistService {
	return &playlistService{repo: repo, musicService: musicService}
}

func (s *playlistService) Create(ctx context.Context, userID uint, req *domain.CreatePlaylistRequest) (*domain.PlaylistDetailResponse, error) {
	name, err := normalizePlaylistName(req.Name)
	if err != nil {
		return nil, err
	}
	description, err := normalizePlaylistDescription(req.Description)
	if err != nil {
		return nil, err
	}
	playlist := &domain.Playlist{UserID: userID, Name: name, Description: description, Revision: 1}
	if err := s.repo.Create(ctx, playlist); err != nil {
		return nil, err
	}
	return s.Get(ctx, userID, playlist.ID)
}

func (s *playlistService) List(ctx context.Context, userID uint, page, pageSize int) ([]*domain.Playlist, int64, error) {
	return s.repo.ListByUserID(ctx, userID, page, pageSize)
}

func (s *playlistService) Get(ctx context.Context, userID, id uint) (*domain.PlaylistDetailResponse, error) {
	playlist, err := s.repo.FindOwnedByID(ctx, id, userID)
	if err != nil {
		return nil, err
	}
	items, err := s.repo.ListItems(ctx, id, userID)
	if err != nil {
		return nil, err
	}
	ids := make([]uint, 0, len(items))
	for _, item := range items {
		ids = append(ids, item.MusicID)
	}
	currentUserID := userID
	musics, err := s.musicService.GetByIDs(ctx, ids, &currentUserID)
	if err != nil {
		return nil, err
	}
	musicByID := make(map[uint]*domain.MusicResponse, len(musics))
	for _, music := range musics {
		musicByID[music.ID] = music
	}
	responseItems := make([]domain.PlaylistItemResponse, 0, len(items))
	for _, item := range items {
		music := musicByID[item.MusicID]
		if music == nil {
			continue
		}
		responseItems = append(responseItems, domain.PlaylistItemResponse{Position: item.Position, Music: music})
	}
	playlist.ItemCount = int64(len(responseItems))
	return &domain.PlaylistDetailResponse{Playlist: *playlist, Items: responseItems}, nil
}

func (s *playlistService) Update(ctx context.Context, userID, id uint, req *domain.UpdatePlaylistRequest) (*domain.PlaylistDetailResponse, error) {
	if req.Name == nil && req.Description == nil {
		return nil, fmt.Errorf("%w: at least one field is required", ErrInvalidPlaylist)
	}
	var name *string
	if req.Name != nil {
		normalized, err := normalizePlaylistName(*req.Name)
		if err != nil {
			return nil, err
		}
		name = &normalized
	}
	var description *string
	if req.Description != nil {
		normalized, err := normalizePlaylistDescription(*req.Description)
		if err != nil {
			return nil, err
		}
		description = &normalized
	}
	if err := s.repo.Update(ctx, id, userID, name, description); err != nil {
		return nil, err
	}
	return s.Get(ctx, userID, id)
}

func (s *playlistService) Delete(ctx context.Context, userID, id uint) error {
	return s.repo.Delete(ctx, id, userID)
}

func (s *playlistService) AddItem(ctx context.Context, userID, id, musicID uint) (*domain.PlaylistDetailResponse, error) {
	if err := s.repo.AddItem(ctx, id, userID, musicID); err != nil {
		return nil, err
	}
	return s.Get(ctx, userID, id)
}

func (s *playlistService) RemoveItem(ctx context.Context, userID, id, musicID uint) (*domain.PlaylistDetailResponse, error) {
	if err := s.repo.RemoveItem(ctx, id, userID, musicID); err != nil {
		return nil, err
	}
	return s.Get(ctx, userID, id)
}

func (s *playlistService) ReorderItems(ctx context.Context, userID, id uint, musicIDs []uint) (*domain.PlaylistDetailResponse, error) {
	if len(musicIDs) > domain.MaxPlaylistItems {
		return nil, repository.ErrPlaylistFull
	}
	if err := s.repo.ReorderItems(ctx, id, userID, musicIDs); err != nil {
		return nil, err
	}
	return s.Get(ctx, userID, id)
}

func normalizePlaylistName(value string) (string, error) {
	value = strings.Join(strings.Fields(strings.TrimSpace(value)), " ")
	if value == "" {
		return "", fmt.Errorf("%w: name is required", ErrInvalidPlaylist)
	}
	if strings.ContainsRune(value, '\x00') || utf8.RuneCountInString(value) > maxPlaylistNameCharacters {
		return "", fmt.Errorf("%w: name exceeds %d characters or contains NUL", ErrInvalidPlaylist, maxPlaylistNameCharacters)
	}
	return value, nil
}

func normalizePlaylistDescription(value string) (string, error) {
	value = strings.TrimSpace(value)
	if strings.ContainsRune(value, '\x00') || utf8.RuneCountInString(value) > maxPlaylistDescriptionCharacters {
		return "", fmt.Errorf("%w: description exceeds %d characters or contains NUL", ErrInvalidPlaylist, maxPlaylistDescriptionCharacters)
	}
	return value, nil
}
