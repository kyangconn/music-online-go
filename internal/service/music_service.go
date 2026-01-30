package service

import (
	"github.com/kyangconn/music-online-web/internal/domain"
	"github.com/kyangconn/music-online-web/internal/repository"
)

type MusicService interface {
	Create(userID uint, req *domain.CreateMusicRequest) (*domain.MusicResponse, error)
	GetByID(id uint, currentUserID *uint) (*domain.MusicResponse, error)
	Search(query string, page, pageSize int, currentUserID *uint) ([]*domain.MusicResponse, int64, error)
	ListByUserID(userID uint, page, pageSize int, currentUserID *uint) ([]*domain.MusicResponse, int64, error)
	Update(id uint, req *domain.UpdateMusicRequest) (*domain.MusicResponse, error)
	Delete(id uint) error
	Like(userID, musicID uint) error
	Unlike(userID, musicID uint) error
	ListLikedByUserID(userID uint, page, pageSize int, currentUserID *uint) ([]*domain.MusicResponse, int64, error)
	// Admin
	AdminDelete(id uint) error
}

type musicService struct {
	repo repository.MusicRepository
}

func NewMusicService(repo repository.MusicRepository) MusicService {
	return &musicService{repo: repo}
}

func (s *musicService) Create(userID uint, req *domain.CreateMusicRequest) (*domain.MusicResponse, error) {
	music := &domain.Music{
		Title:       req.Title,
		Artist:      req.Artist,
		Intro:       req.Intro,
		Img:         req.Img,
		Type:        req.Type,
		IssuingDate: req.IssuingDate,
		UserID:      userID,
		AlbumID:     req.AlbumID,
	}

	if err := s.repo.Create(music); err != nil {
		return nil, err
	}

	return music.ToResponse(), nil
}

func (s *musicService) GetByID(id uint, currentUserID *uint) (*domain.MusicResponse, error) {
	music, err := s.repo.FindByID(id)
	if err != nil {
		return nil, err
	}

	resp := music.ToResponse()
	if err := s.enrichMusicResponse(resp, currentUserID); err != nil {
		return nil, err
	}
	return resp, nil
}

func (s *musicService) Search(query string, page, pageSize int, currentUserID *uint) ([]*domain.MusicResponse, int64, error) {
	musics, total, err := s.repo.Search(query, page, pageSize)
	if err != nil {
		return nil, 0, err
	}

	var responses []*domain.MusicResponse
	for _, m := range musics {
		resp := m.ToResponse()
		if err := s.enrichMusicResponse(resp, currentUserID); err != nil {
			return nil, 0, err
		}
		responses = append(responses, resp)
	}

	return responses, total, nil
}

func (s *musicService) ListByUserID(userID uint, page, pageSize int, currentUserID *uint) ([]*domain.MusicResponse, int64, error) {
	musics, total, err := s.repo.ListByUserID(userID, page, pageSize)
	if err != nil {
		return nil, 0, err
	}

	var responses []*domain.MusicResponse
	for _, m := range musics {
		resp := m.ToResponse()
		if err := s.enrichMusicResponse(resp, currentUserID); err != nil {
			return nil, 0, err
		}
		responses = append(responses, resp)
	}

	return responses, total, nil
}

func (s *musicService) Update(id uint, req *domain.UpdateMusicRequest) (*domain.MusicResponse, error) {
	music, err := s.repo.FindByID(id)
	if err != nil {
		return nil, err
	}

	if req.Title != nil {
		music.Title = *req.Title
	}
	if req.Artist != nil {
		music.Artist = *req.Artist
	}
	if req.Intro != nil {
		music.Intro = *req.Intro
	}
	if req.Img != nil {
		music.Img = *req.Img
	}
	if req.Type != nil {
		music.Type = *req.Type
	}
	if req.IssuingDate != nil {
		music.IssuingDate = *req.IssuingDate
	}
	if req.AlbumID != nil {
		music.AlbumID = req.AlbumID
	}

	if err := s.repo.Update(music); err != nil {
		return nil, err
	}

	return music.ToResponse(), nil
}

func (s *musicService) Delete(id uint) error {
	return s.repo.Delete(id)
}

func (s *musicService) Like(userID, musicID uint) error {
	// Check if music exists
	if _, err := s.repo.FindByID(musicID); err != nil {
		return err
	}
	return s.repo.LikeMusic(userID, musicID)
}

func (s *musicService) Unlike(userID, musicID uint) error {
	return s.repo.UnlikeMusic(userID, musicID)
}

func (s *musicService) ListLikedByUserID(userID uint, page, pageSize int, currentUserID *uint) ([]*domain.MusicResponse, int64, error) {
	musics, total, err := s.repo.ListLikedByUserID(userID, page, pageSize)
	if err != nil {
		return nil, 0, err
	}

	var responses []*domain.MusicResponse
	for _, m := range musics {
		resp := m.ToResponse()
		if err := s.enrichMusicResponse(resp, currentUserID); err != nil {
			return nil, 0, err
		}
		responses = append(responses, resp)
	}

	return responses, total, nil
}

// enrichMusicResponse populates IsLiked and LikeCount.
// Note: This performs N+1 queries. For high traffic, consider batching or joining in repository.
func (s *musicService) enrichMusicResponse(resp *domain.MusicResponse, currentUserID *uint) error {
	count, err := s.repo.CountLikes(resp.ID)
	if err != nil {
		return err
	}
	resp.LikeCount = count

	if currentUserID != nil {
		liked, err := s.repo.IsLiked(*currentUserID, resp.ID)
		if err != nil {
			return err
		}
		resp.IsLiked = liked
	} else {
		resp.IsLiked = false
	}
	return nil
}
