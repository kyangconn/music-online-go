// Package service music_service.go - 音乐服务层
// 包含音乐的创建、查询、更新、删除、收藏及文件上传等业务逻辑
package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/kyangconn/music-online-go/internal/config"
	"github.com/kyangconn/music-online-go/internal/domain"
	pklog "github.com/kyangconn/music-online-go/internal/pkg/log"
	"github.com/kyangconn/music-online-go/internal/repository"
)

var (
	ErrForbidden        = errors.New("forbidden")
	ErrMediaNotFound    = errors.New("media file not found")
	ErrInvalidMediaFile = errors.New("invalid media file")
)

// MusicService defines music business logic operations.
type MusicService interface {
	Create(ctx context.Context, userID uint, req *domain.CreateMusicRequest) (*domain.MusicResponse, error)
	GetByID(ctx context.Context, id uint, currentUserID *uint) (*domain.MusicResponse, error)
	Search(ctx context.Context, params *domain.MusicSearchParams, currentUserID *uint) ([]*domain.MusicResponse, int64, error)
	ListFilterOptions(ctx context.Context) (*domain.MusicFilterOptions, error)
	CheckDuplicates(ctx context.Context, userID uint, role string, req *domain.MusicDuplicateCheckRequest) (*domain.MusicDuplicateCheckResponse, error)
	ListByUserID(ctx context.Context, userID uint, page, pageSize int, currentUserID *uint) ([]*domain.MusicResponse, int64, error)
	Update(ctx context.Context, userID uint, role string, id uint, req *domain.UpdateMusicRequest) (*domain.MusicResponse, error)
	Delete(ctx context.Context, userID uint, role string, id uint) error
	Like(ctx context.Context, userID, musicID uint) error
	Unlike(ctx context.Context, userID, musicID uint) error
	ListLikedByUserID(ctx context.Context, userID uint, page, pageSize int, currentUserID *uint) ([]*domain.MusicResponse, int64, error)
	UploadFiles(ctx context.Context, userID uint, role string, id uint, audioHeader, coverHeader *multipart.FileHeader) (*domain.MusicResponse, error)
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
		Album:       req.Album,
		Year:        req.Year,
		TrackNumber: req.TrackNumber,
		Genre:       req.Genre,
		Duration:    req.Duration,
		Intro:       req.Intro,
		Img:         "",
		Path:        "",
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

func (s *musicService) Search(ctx context.Context, params *domain.MusicSearchParams, currentUserID *uint) ([]*domain.MusicResponse, int64, error) {
	musics, total, err := s.repo.Search(ctx, params)
	if err != nil {
		return nil, 0, err
	}

	responses, err := s.toEnrichedResponses(ctx, musics, currentUserID)
	if err != nil {
		return nil, 0, err
	}

	return responses, total, nil
}

func (s *musicService) ListFilterOptions(ctx context.Context) (*domain.MusicFilterOptions, error) {
	return s.repo.ListFilterOptions(ctx)
}

func (s *musicService) CheckDuplicates(
	ctx context.Context,
	userID uint,
	role string,
	req *domain.MusicDuplicateCheckRequest,
) (*domain.MusicDuplicateCheckResponse, error) {
	response := &domain.MusicDuplicateCheckResponse{
		MetadataMatches:   []*domain.MusicResponse{},
		SuggestedMetadata: req.Metadata(),
	}

	var exact *domain.Music
	fileHash := strings.ToLower(strings.TrimSpace(req.FileHash))
	if fileHash != "" {
		match, err := s.repo.FindByFileHash(ctx, fileHash)
		if err != nil && !errors.Is(err, repository.ErrMusicNotFound) {
			return nil, err
		}
		exact = match
		if exact != nil {
			response.ExactMatch = exact.ToResponse()
			if canManageMusic(exact, userID, role) {
				response.Enrichment = buildMetadataEnrichment(exact, req.Metadata())
			}
		}
	}

	matches, err := s.repo.FindByTitleAndArtist(ctx, req.Title, req.Artist, 5)
	if err != nil {
		return nil, err
	}
	for _, match := range matches {
		if exact != nil && match.ID == exact.ID {
			continue
		}
		response.MetadataMatches = append(response.MetadataMatches, match.ToResponse())
	}

	response.SuggestedMetadata = buildSuggestedMetadata(req.Metadata(), exact, matches)
	return response, nil
}

func (s *musicService) ListByUserID(ctx context.Context, userID uint, page, pageSize int, currentUserID *uint) ([]*domain.MusicResponse, int64, error) {
	musics, total, err := s.repo.ListByUserID(ctx, userID, page, pageSize)
	if err != nil {
		return nil, 0, err
	}

	responses, err := s.toEnrichedResponses(ctx, musics, currentUserID)
	if err != nil {
		return nil, 0, err
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
	if req.Album != nil {
		music.Album = *req.Album
	}
	if req.Year != nil {
		music.Year = *req.Year
	}
	if req.TrackNumber != nil {
		music.TrackNumber = *req.TrackNumber
	}
	if req.Genre != nil {
		music.Genre = *req.Genre
	}
	if req.Duration != nil {
		music.Duration = *req.Duration
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

	responses, err := s.toEnrichedResponses(ctx, musics, currentUserID)
	if err != nil {
		return nil, 0, err
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

	// Path traversal protection: ensure resolved path stays within upload dir
	absPath, err := filepath.Abs(filepath.Clean(music.Path))
	if err != nil {
		return "", fmt.Errorf("invalid audio path: %w", err)
	}
	uploadDir := filepath.Clean(config.AppConfig.Server.UploadDir)
	if !strings.HasPrefix(absPath, uploadDir+string(filepath.Separator)) && absPath != uploadDir {
		return "", ErrMediaNotFound
	}

	return absPath, nil
}

func (s *musicService) GetCoverPath(ctx context.Context, id uint) (string, error) {
	music, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return "", err
	}
	if music.Img == "" {
		return "", ErrMediaNotFound
	}

	// Path traversal protection: ensure resolved path stays within upload dir
	absPath, err := filepath.Abs(filepath.Clean(music.Img))
	if err != nil {
		return "", fmt.Errorf("invalid cover path: %w", err)
	}
	uploadDir := filepath.Clean(config.AppConfig.Server.UploadDir)
	if !strings.HasPrefix(absPath, uploadDir+string(filepath.Separator)) && absPath != uploadDir {
		return "", ErrMediaNotFound
	}

	return absPath, nil
}

// toEnrichedResponses 批量转换并填充音乐响应列表。
// 统一 Search / ListByUserID / ListLikedByUserID 的「ToResponse + enrich」循环，避免重复逻辑。
func (s *musicService) toEnrichedResponses(ctx context.Context, musics []*domain.Music, currentUserID *uint) ([]*domain.MusicResponse, error) {
	responses := make([]*domain.MusicResponse, 0, len(musics))
	for _, m := range musics {
		resp := m.ToResponse()
		if err := s.enrichMusicResponse(ctx, resp, currentUserID); err != nil {
			return nil, err
		}
		responses = append(responses, resp)
	}
	return responses, nil
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
func (s *musicService) UploadFiles(ctx context.Context, userID uint, role string, id uint, audioHeader, coverHeader *multipart.FileHeader) (*domain.MusicResponse, error) {
	music, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if !canManageMusic(music, userID, role) {
		return nil, ErrForbidden
	}
	if err := validateUploadedAudioFile(audioHeader); err != nil {
		return nil, err
	}
	if err := validateUploadedCoverFile(coverHeader); err != nil {
		return nil, err
	}

	uploadDir := config.AppConfig.Server.UploadDir
	musicDir := filepath.Join(uploadDir, strconv.FormatUint(uint64(id), 10))
	if err := os.MkdirAll(musicDir, 0700); err != nil {
		return nil, fmt.Errorf("failed to create upload directory: %w", err)
	}

	if audioHeader != nil {
		tmpPath, finalPath, fileHash, err := saveUploadedMediaFile(musicDir, "audio", audioHeader, true)
		if err != nil {
			return nil, err
		}
		// Remove old file to avoid orphaned old-extension files
		if music.Path != "" && music.Path != finalPath {
			cleanupUploadedFiles([]string{music.Path})
		}
		// Atomically rename temp file to final path
		if err := os.Rename(tmpPath, finalPath); err != nil {
			return nil, fmt.Errorf("failed to finalize audio file: %w", err)
		}
		music.Path = finalPath
		music.FileHash = fileHash
	}

	if coverHeader != nil {
		tmpPath, finalPath, _, err := saveUploadedMediaFile(musicDir, "cover", coverHeader, false)
		if err != nil {
			return nil, err
		}
		// Remove old file to avoid orphaned old-extension files
		if music.Img != "" && music.Img != finalPath {
			cleanupUploadedFiles([]string{music.Img})
		}
		// Atomically rename temp file to final path
		if err := os.Rename(tmpPath, finalPath); err != nil {
			return nil, fmt.Errorf("failed to finalize cover file: %w", err)
		}
		music.Img = finalPath
	}

	if err := s.repo.Update(ctx, music); err != nil {
		return nil, err
	}

	return music.ToResponse(), nil
}

// saveUploadedMediaFile saves the uploaded file to a temp file first, returning
// the temp path, final path (with extension), and optional file hash.
// The caller is responsible for the atomic os.Rename from tmpPath to finalPath.
func saveUploadedMediaFile(dir, baseName string, header *multipart.FileHeader, calculateHash bool) (tmpPath, finalPath string, fileHash string, err error) {
	ext := filepath.Ext(header.Filename)
	finalPath = filepath.Join(dir, baseName+ext)
	tmpPath = filepath.Join(dir, baseName+".tmp")

	src, err := header.Open()
	if err != nil {
		return "", "", "", fmt.Errorf("failed to open %s file: %w", baseName, err)
	}
	defer func() {
		if cerr := src.Close(); cerr != nil && !errors.Is(cerr, io.EOF) && !errors.Is(cerr, io.ErrUnexpectedEOF) {
			pklog.Errorf("Failed to close %s source: %v", baseName, cerr)
		}
	}()

	dst, err := os.Create(filepath.Clean(tmpPath))
	if err != nil {
		return "", "", "", fmt.Errorf("failed to create %s file: %w", baseName, err)
	}
	saved := false
	defer func() {
		if cerr := dst.Close(); cerr != nil {
			pklog.Errorf("Failed to close %s destination: %v", baseName, cerr)
		}
		if !saved {
			cleanupUploadedFiles([]string{tmpPath})
		}
	}()

	var writer io.Writer = dst
	hasher := sha256.New()
	if calculateHash {
		writer = io.MultiWriter(dst, hasher)
	}
	if _, err := io.Copy(writer, src); err != nil {
		return "", "", "", fmt.Errorf("failed to save %s file: %w", baseName, err)
	}
	saved = true
	if calculateHash {
		fileHash = hex.EncodeToString(hasher.Sum(nil))
	}
	return tmpPath, finalPath, fileHash, nil
}

func buildMetadataEnrichment(existing *domain.Music, incoming domain.MusicMetadata) *domain.UpdateMusicRequest {
	patch := &domain.UpdateMusicRequest{}
	changed := false
	if existing.Album == "" && incoming.Album != "" {
		patch.Album = &incoming.Album
		changed = true
	}
	if existing.Year == 0 && incoming.Year > 0 {
		patch.Year = &incoming.Year
		changed = true
	}
	if existing.TrackNumber == 0 && incoming.TrackNumber > 0 {
		patch.TrackNumber = &incoming.TrackNumber
		changed = true
	}
	if existing.Genre == "" && incoming.Genre != "" {
		patch.Genre = &incoming.Genre
		changed = true
	}
	if existing.Duration == 0 && incoming.Duration > 0 {
		patch.Duration = &incoming.Duration
		changed = true
	}
	if !changed {
		return nil
	}
	return patch
}

func buildSuggestedMetadata(incoming domain.MusicMetadata, exact *domain.Music, matches []*domain.Music) domain.MusicMetadata {
	best := exact
	for _, candidate := range matches {
		if best == nil || metadataCompleteness(candidate) > metadataCompleteness(best) {
			best = candidate
		}
	}
	if best == nil {
		return incoming
	}
	if incoming.Album == "" {
		incoming.Album = best.Album
	}
	if incoming.Year == 0 {
		incoming.Year = best.Year
	}
	if incoming.TrackNumber == 0 {
		incoming.TrackNumber = best.TrackNumber
	}
	if incoming.Genre == "" {
		incoming.Genre = best.Genre
	}
	if incoming.Duration == 0 {
		incoming.Duration = best.Duration
	}
	return incoming
}

func metadataCompleteness(music *domain.Music) int {
	score := 0
	if music.Album != "" {
		score++
	}
	if music.Year > 0 {
		score++
	}
	if music.TrackNumber > 0 {
		score++
	}
	if music.Genre != "" {
		score++
	}
	if music.Duration > 0 {
		score++
	}
	return score
}

func cleanupUploadedFiles(paths []string) {
	for _, path := range paths {
		if path == "" {
			continue
		}
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			pklog.Errorf("Failed to clean up uploaded file %s: %v", path, err)
		}
	}
}
