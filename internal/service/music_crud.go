// Package service music_crud.go - 音乐更新与删除
package service

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/kyangconn/music-online-go/internal/domain"
	pklog "github.com/kyangconn/music-online-go/internal/pkg/log"
)

func (s *musicService) Update(ctx context.Context, userID uint, role string, id uint, req *domain.UpdateMusicRequest) (*domain.MusicResponse, error) {
	music, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if !canManageMusic(music, userID, role) {
		return nil, ErrForbidden
	}
	if err := s.guardReadOnlyMediaSource(ctx, music, role); err != nil {
		return nil, err
	}

	if _, err := applyUpdateMusicMetadata(music, req); err != nil {
		return nil, err
	}
	if req.Intro != nil {
		music.Intro = *req.Intro
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

	return s.toResponse(music), nil
}

func (s *musicService) Delete(ctx context.Context, userID uint, role string, id uint) error {
	music, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return err
	}
	if !canManageMusic(music, userID, role) {
		return ErrForbidden
	}
	if err := s.guardReadOnlyMediaSource(ctx, music, role); err != nil {
		return err
	}

	return s.deleteMusic(ctx, id)
}

func (s *musicService) deleteMusic(ctx context.Context, id uint) error {
	if err := s.repo.Delete(ctx, id); err != nil {
		return err
	}
	cleanupMusicUploadDirectoryAt(s.serverConfig.UploadDir, id)
	return nil
}

func cleanupMusicUploadDirectoryAt(uploadDir string, id uint) {
	if strings.TrimSpace(uploadDir) == "" {
		return
	}
	dir := filepath.Join(uploadDir, strconv.FormatUint(uint64(id), 10))
	if err := os.RemoveAll(dir); err != nil {
		pklog.Errorf("Failed to clean up upload directory %s: %v", dir, err)
	}
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
