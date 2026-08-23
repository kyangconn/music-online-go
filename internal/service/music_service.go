// Package service music_service.go - 音乐服务层
// 包含音乐的创建、查询、更新、删除、收藏及文件上传等业务逻辑
package service

import (
	"context"
	"errors"
	"mime/multipart"

	"github.com/kyangconn/music-online-go/internal/config"
	"github.com/kyangconn/music-online-go/internal/domain"
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
	GetByIDs(ctx context.Context, ids []uint, currentUserID *uint) ([]*domain.MusicResponse, error)
	Search(ctx context.Context, params *domain.MusicSearchParams, currentUserID *uint) ([]*domain.MusicResponse, int64, error)
	FindByMusicBrainzRecordingID(ctx context.Context, recordingID string, currentUserID *uint) (*domain.MusicResponse, error)
	FindMetadataCandidates(ctx context.Context, metadata domain.MusicMetadata, currentUserID *uint) ([]*domain.MusicResponse, bool, error)
	ListFilterOptions(ctx context.Context) (*domain.MusicFilterOptions, error)
	CountWithMetadata(ctx context.Context) (int64, error)
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
	GetUploadPolicy() UploadPolicy
	UploadBodyLimit() int64
	// Admin
	AdminDelete(ctx context.Context, id uint) error
}

// MediaStorage is the complete physical-media boundary required by music
// mutations. Keeping persistence and resolution together prevents uploads
// from silently bypassing the MediaFile source-of-truth table.
type MediaStorage interface {
	MediaPathResolver
	ManagedMediaSourcePersister
}

type musicService struct {
	repo         repository.MusicRepository
	storage      MediaStorage
	serverConfig config.ServerConfig
	presenter    musicPresenter
	uploadPolicy UploadPolicy
	presetRepo   repository.PresetRepository
	analysisRepo repository.MusicAnalysisRepository
	analyzer     MusicAnalysisScheduler
}

// NewMusicService accepts only the fully wired production dependency set. The
// configuration snapshot must already have passed config.Validate.
func NewMusicService(
	repo repository.MusicRepository,
	storage MediaStorage,
	cfg config.Config,
	presetRepo repository.PresetRepository,
	analysisRepo repository.MusicAnalysisRepository,
	analyzer MusicAnalysisScheduler,
) MusicService {
	return &musicService{
		repo:         repo,
		storage:      storage,
		serverConfig: cfg.Server,
		presenter:    newMusicPresenter(cfg.Access, cfg.JWT),
		uploadPolicy: UploadPolicyFromServerConfig(cfg.Server),
		presetRepo:   presetRepo,
		analysisRepo: analysisRepo,
		analyzer:     analyzer,
	}
}

func (s *musicService) GetUploadPolicy() UploadPolicy {
	return s.uploadPolicy
}

func (s *musicService) UploadBodyLimit() int64 {
	return s.uploadPolicy.MaxAudioSizeBytes + s.uploadPolicy.MaxCoverSizeBytes + multipartOverheadBytes
}

func (s *musicService) Create(ctx context.Context, userID uint, req *domain.CreateMusicRequest) (*domain.MusicResponse, error) {
	musicType := req.Type
	if musicType == "" {
		musicType = domain.MusicTypeSingle
	}

	music := &domain.Music{
		Intro:       req.Intro,
		Img:         "",
		Path:        "",
		Type:        musicType,
		IssuingDate: req.IssuingDate,
		UserID:      userID,
		AlbumID:     req.AlbumID,
	}
	if err := applyCreateMusicMetadata(music, req); err != nil {
		return nil, err
	}

	if err := s.repo.Create(ctx, music); err != nil {
		return nil, err
	}

	return s.toResponse(music), nil
}

func canManageMusic(music *domain.Music, userID uint, role string) bool {
	return role == "admin" || music.UserID == userID
}

func (s *musicService) guardReadOnlyMediaSource(ctx context.Context, music *domain.Music, role string) error {
	if role == "admin" {
		return nil
	}
	// SourceReadOnly is retained for pre-migration rows. New records use the
	// physical MediaFile table, which also catches a read-only duplicate added
	// after a user originally uploaded the logical track.
	if music.SourceReadOnly {
		return ErrForbidden
	}
	readOnly, err := s.storage.HasReadOnlyMediaSource(ctx, music.ID)
	if err != nil {
		return err
	}
	if readOnly {
		return ErrForbidden
	}
	return nil
}
