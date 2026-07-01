// Package service music_service.go - 音乐服务层
// 包含音乐的创建、查询、更新、删除、收藏及文件上传等业务逻辑
package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"os"
	"path/filepath"
	"strconv"

	"github.com/kyangconn/music-online-go/internal/config"
	"github.com/kyangconn/music-online-go/internal/domain"
	pklog "github.com/kyangconn/music-online-go/internal/pkg/log"
	"github.com/kyangconn/music-online-go/internal/repository"
)

var (
	ErrForbidden     = errors.New("forbidden")
	ErrMediaNotFound = errors.New("media file not found")
)

// MusicService defines music business logic operations.
type MusicService interface {
	Create(ctx context.Context, userID uint, req *domain.CreateMusicRequest) (*domain.MusicResponse, error)
	GetByID(ctx context.Context, id uint, currentUserID *uint) (*domain.MusicResponse, error)
	Search(ctx context.Context, query string, page, pageSize int, currentUserID *uint) ([]*domain.MusicResponse, int64, error)
	ListByUserID(ctx context.Context, userID uint, page, pageSize int, currentUserID *uint) ([]*domain.MusicResponse, int64, error)
	Update(ctx context.Context, userID uint, role string, id uint, req *domain.UpdateMusicRequest) (*domain.MusicResponse, error)
	Delete(ctx context.Context, userID uint, role string, id uint) error
	Like(ctx context.Context, userID, musicID uint) error
	Unlike(ctx context.Context, userID, musicID uint) error
	ListLikedByUserID(ctx context.Context, userID uint, page, pageSize int, currentUserID *uint) ([]*domain.MusicResponse, int64, error)
	UploadFiles(ctx context.Context, id uint, audioHeader, coverHeader *multipart.FileHeader) (*domain.MusicResponse, error)
	GetAudioPath(ctx context.Context, id uint) (string, error)
	GetCoverPath(ctx context.Context, id uint) (string, error)
	// Admin
	AdminDelete(ctx context.Context, id uint) error
}
type musicService struct {
	repo repository.MusicRepository
}

func NewMusicService(repo repository.MusicRepository) MusicService {
	return &musicService{repo: repo}
}

func (s *musicService) Create(ctx context.Context, userID uint, req *domain.CreateMusicRequest) (*domain.MusicResponse, error) {
	musicType := req.Type
	if musicType == "" {
		musicType = domain.MusicTypeSingle
	}

	music := &domain.Music{
		Title:       req.Title,
		Artist:      req.Artist,
		Intro:       req.Intro,
		Img:         req.Img,
		Path:        req.Path,
		Type:        musicType,
		IssuingDate: req.IssuingDate,
		UserID:      userID,
		AlbumID:     req.AlbumID,
	}

	if err := s.repo.Create(ctx, music); err != nil {
		return nil, err
	}

	return music.ToResponse(), nil
}

func (s *musicService) GetByID(ctx context.Context, id uint, currentUserID *uint) (*domain.MusicResponse, error) {
	music, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	resp := music.ToResponse()
	if err := s.enrichMusicResponse(ctx, resp, currentUserID); err != nil {
		return nil, err
	}
	return resp, nil
}

func (s *musicService) Search(ctx context.Context, query string, page, pageSize int, currentUserID *uint) ([]*domain.MusicResponse, int64, error) {
	musics, total, err := s.repo.Search(ctx, query, page, pageSize)
	if err != nil {
		return nil, 0, err
	}

	var responses []*domain.MusicResponse
	for _, m := range musics {
		resp := m.ToResponse()
		if err := s.enrichMusicResponse(ctx, resp, currentUserID); err != nil {
			return nil, 0, err
		}
		responses = append(responses, resp)
	}

	return responses, total, nil
}

func (s *musicService) ListByUserID(ctx context.Context, userID uint, page, pageSize int, currentUserID *uint) ([]*domain.MusicResponse, int64, error) {
	musics, total, err := s.repo.ListByUserID(ctx, userID, page, pageSize)
	if err != nil {
		return nil, 0, err
	}

	var responses []*domain.MusicResponse
	for _, m := range musics {
		resp := m.ToResponse()
		if err := s.enrichMusicResponse(ctx, resp, currentUserID); err != nil {
			return nil, 0, err
		}
		responses = append(responses, resp)
	}

	return responses, total, nil
}

func (s *musicService) Update(ctx context.Context, userID uint, role string, id uint, req *domain.UpdateMusicRequest) (*domain.MusicResponse, error) {
	music, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if !canManageMusic(music, userID, role) {
		return nil, ErrForbidden
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

	if err := s.repo.Update(ctx, music); err != nil {
		return nil, err
	}

	return music.ToResponse(), nil
}

func (s *musicService) Delete(ctx context.Context, userID uint, role string, id uint) error {
	music, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return err
	}
	if !canManageMusic(music, userID, role) {
		return ErrForbidden
	}

	return s.repo.Delete(ctx, id)
}

func (s *musicService) Like(ctx context.Context, userID, musicID uint) error {
	// Check if music exists
	if _, err := s.repo.FindByID(ctx, musicID); err != nil {
		return err
	}
	return s.repo.LikeMusic(ctx, userID, musicID)
}

func (s *musicService) Unlike(ctx context.Context, userID, musicID uint) error {
	return s.repo.UnlikeMusic(ctx, userID, musicID)
}

func (s *musicService) ListLikedByUserID(ctx context.Context, userID uint, page, pageSize int, currentUserID *uint) ([]*domain.MusicResponse, int64, error) {
	musics, total, err := s.repo.ListLikedByUserID(ctx, userID, page, pageSize)
	if err != nil {
		return nil, 0, err
	}

	var responses []*domain.MusicResponse
	for _, m := range musics {
		resp := m.ToResponse()
		if err := s.enrichMusicResponse(ctx, resp, currentUserID); err != nil {
			return nil, 0, err
		}
		responses = append(responses, resp)
	}

	return responses, total, nil
}

func (s *musicService) GetAudioPath(ctx context.Context, id uint) (string, error) {
	music, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return "", err
	}
	if music.Path == "" {
		return "", ErrMediaNotFound
	}
	return music.Path, nil
}

func (s *musicService) GetCoverPath(ctx context.Context, id uint) (string, error) {
	music, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return "", err
	}
	if music.Img == "" {
		return "", ErrMediaNotFound
	}
	return music.Img, nil
}

func canManageMusic(music *domain.Music, userID uint, role string) bool {
	return role == "admin" || music.UserID == userID
}

// enrichMusicResponse populates IsLiked and LikeCount.
// Note: This performs N+1 queries. For high traffic, consider batching or joining in repository.
func (s *musicService) enrichMusicResponse(ctx context.Context, resp *domain.MusicResponse, currentUserID *uint) error {
	count, err := s.repo.CountLikes(ctx, resp.ID)
	if err != nil {
		return err
	}
	resp.LikeCount = count

	if currentUserID != nil {
		liked, err := s.repo.IsLiked(ctx, *currentUserID, resp.ID)
		if err != nil {
			return err
		}
		resp.IsLiked = liked
	} else {
		resp.IsLiked = false
	}
	return nil
}

// UploadFiles 上传音频和封面文件到已有音乐记录
func (s *musicService) UploadFiles(ctx context.Context, id uint, audioHeader, coverHeader *multipart.FileHeader) (*domain.MusicResponse, error) {
	music, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	uploadDir := config.AppConfig.Server.UploadDir
	musicDir := filepath.Join(uploadDir, strconv.FormatUint(uint64(id), 10))
	if err := os.MkdirAll(musicDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create upload directory: %w", err)
	}

	if audioHeader != nil {
		dest, err := saveUploadedMediaFile(musicDir, "audio", audioHeader)
		if err != nil {
			return nil, err
		}
		music.Path = dest
	}

	if coverHeader != nil {
		dest, err := saveUploadedMediaFile(musicDir, "cover", coverHeader)
		if err != nil {
			return nil, err
		}
		music.Img = dest
	}

	if err := s.repo.Update(ctx, music); err != nil {
		return nil, err
	}

	return music.ToResponse(), nil
}

func saveUploadedMediaFile(dir, baseName string, header *multipart.FileHeader) (string, error) {
	ext := filepath.Ext(header.Filename)
	dest := filepath.Join(dir, baseName+ext)

	src, err := header.Open()
	if err != nil {
		return "", fmt.Errorf("failed to open %s file: %w", baseName, err)
	}
	defer func() {
		if err := src.Close(); err != nil {
			pklog.Errorf("Failed to close %s source: %v", baseName, err)
		}
	}()

	dst, err := os.Create(dest)
	if err != nil {
		return "", fmt.Errorf("failed to create %s file: %w", baseName, err)
	}
	defer func() {
		if err := dst.Close(); err != nil {
			pklog.Errorf("Failed to close %s destination: %v", baseName, err)
		}
	}()

	if _, err := io.Copy(dst, src); err != nil {
		return "", fmt.Errorf("failed to save %s file: %w", baseName, err)
	}
	return dest, nil
}
