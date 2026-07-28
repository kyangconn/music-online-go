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
	"time"

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
	SetAnalysisScheduler(scheduler MusicAnalysisScheduler)
	// Admin
	AdminDelete(ctx context.Context, id uint) error
}
type musicService struct {
	repo         repository.MusicRepository
	pathResolver MediaPathResolver
	serverConfig config.ServerConfig
	presenter    musicPresenter
	uploadPolicy UploadPolicy
	presetRepo   repository.PresetRepository
	analysisRepo repository.MusicAnalysisRepository
	analyzer     MusicAnalysisScheduler
}

func NewMusicService(repo repository.MusicRepository, pathResolvers ...MediaPathResolver) MusicService {
	var pathResolver MediaPathResolver
	if len(pathResolvers) > 0 {
		pathResolver = pathResolvers[0]
	}
	return NewMusicServiceWithConfig(repo, pathResolver, config.AppConfig)
}

func NewMusicServiceWithConfig(
	repo repository.MusicRepository,
	pathResolver MediaPathResolver,
	cfg *config.Config,
	presetRepositories ...repository.PresetRepository,
) MusicService {
	return NewMusicServiceWithAnalysis(repo, pathResolver, cfg, firstPresetRepository(presetRepositories), nil)
}

func NewMusicServiceWithAnalysis(
	repo repository.MusicRepository,
	pathResolver MediaPathResolver,
	cfg *config.Config,
	presetRepo repository.PresetRepository,
	analysisRepo repository.MusicAnalysisRepository,
) MusicService {
	serverConfig := config.ServerConfig{
		UploadDir:      "uploads",
		MaxAudioSizeMB: config.DefaultMaxAudioSizeMB,
		MaxCoverSizeMB: config.DefaultMaxCoverSizeMB,
	}
	if cfg != nil {
		serverConfig = cfg.Server
	}
	return &musicService{
		repo:         repo,
		pathResolver: pathResolver,
		serverConfig: serverConfig,
		presenter:    newMusicPresenter(cfg),
		uploadPolicy: UploadPolicyFromServerConfig(serverConfig),
		presetRepo:   presetRepo,
		analysisRepo: analysisRepo,
	}
}

func firstPresetRepository(values []repository.PresetRepository) repository.PresetRepository {
	if len(values) == 0 {
		return nil
	}
	return values[0]
}

func (s *musicService) SetAnalysisScheduler(scheduler MusicAnalysisScheduler) {
	s.analyzer = scheduler
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

func (s *musicService) GetByID(ctx context.Context, id uint, currentUserID *uint) (*domain.MusicResponse, error) {
	music, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	resp := s.toResponse(music)
	if err := s.enrichMusicResponse(ctx, resp, currentUserID); err != nil {
		return nil, err
	}
	return resp, nil
}

func (s *musicService) GetByIDs(ctx context.Context, ids []uint, currentUserID *uint) ([]*domain.MusicResponse, error) {
	musics, err := s.repo.FindByIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	return s.toEnrichedResponses(ctx, musics, currentUserID)
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

func (s *musicService) FindByMusicBrainzRecordingID(ctx context.Context, recordingID string, currentUserID *uint) (*domain.MusicResponse, error) {
	normalized, err := normalizeMBID("musicbrainz_recording_id", recordingID)
	if err != nil {
		return nil, err
	}
	music, err := s.repo.FindByMusicBrainzRecordingID(ctx, normalized)
	if err != nil {
		return nil, err
	}
	resp := s.toResponse(music)
	if err := s.enrichMusicResponse(ctx, resp, currentUserID); err != nil {
		return nil, err
	}
	return resp, nil
}

func (s *musicService) FindMetadataCandidates(ctx context.Context, metadata domain.MusicMetadata, currentUserID *uint) ([]*domain.MusicResponse, bool, error) {
	probe, err := normalizedMusicMetadata(metadata)
	if err != nil {
		return nil, false, err
	}

	stableMatches, err := s.repo.FindByStableMetadataIDs(ctx, probe.MusicBrainzRecordingID, probe.MusicBrainzTrackID, 5)
	if err != nil {
		return nil, false, err
	}
	if len(stableMatches) > 0 {
		responses, err := s.toEnrichedResponses(ctx, stableMatches, currentUserID)
		return responses, true, err
	}

	textMatches, err := s.repo.FindByTitleAndArtist(ctx, probe.Title, probe.Artist, 5)
	if err != nil {
		return nil, false, err
	}
	responses, err := s.toEnrichedResponses(ctx, textMatches, currentUserID)
	return responses, false, err
}

func (s *musicService) ListFilterOptions(ctx context.Context) (*domain.MusicFilterOptions, error) {
	return s.repo.ListFilterOptions(ctx)
}

func (s *musicService) CountWithMetadata(ctx context.Context) (int64, error) {
	return s.repo.CountWithMetadata(ctx)
}

func (s *musicService) CheckDuplicates(
	ctx context.Context,
	userID uint,
	role string,
	req *domain.MusicDuplicateCheckRequest,
) (*domain.MusicDuplicateCheckResponse, error) {
	incoming, err := normalizedMusicMetadata(req.Metadata())
	if err != nil {
		return nil, err
	}
	response := &domain.MusicDuplicateCheckResponse{
		MetadataMatches:   []*domain.MusicResponse{},
		SuggestedMetadata: incoming,
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
			response.ExactMatch = s.toResponse(exact)
			if canManageMusic(exact, userID, role) {
				response.Enrichment = buildMetadataEnrichment(exact, incoming)
			}
		}
	}

	matches, err := s.repo.FindByStableMetadataIDs(ctx, incoming.MusicBrainzRecordingID, incoming.MusicBrainzTrackID, 5)
	if err != nil {
		return nil, err
	}
	textMatches, err := s.repo.FindByTitleAndArtist(ctx, incoming.Title, incoming.Artist, 5)
	if err != nil {
		return nil, err
	}
	seenMatches := make(map[uint]struct{}, len(matches)+len(textMatches))
	for _, match := range append(matches, textMatches...) {
		if exact != nil && match.ID == exact.ID {
			continue
		}
		if _, exists := seenMatches[match.ID]; exists {
			continue
		}
		seenMatches[match.ID] = struct{}{}
		response.MetadataMatches = append(response.MetadataMatches, s.toResponse(match))
	}

	response.SuggestedMetadata = buildSuggestedMetadata(incoming, exact, matches)
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

func cleanupMusicUploadDirectory(id uint) {
	if config.AppConfig == nil || config.AppConfig.Server.UploadDir == "" {
		return
	}
	cleanupMusicUploadDirectoryAt(config.AppConfig.Server.UploadDir, id)
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
	if music.MediaRelativePath != "" && s.pathResolver != nil {
		return s.pathResolver.ResolveMusicPath(ctx, music)
	}
	return secureManagedMediaPathAt(s.serverConfig.UploadDir, music.Path)
}

func (s *musicService) GetCoverPath(ctx context.Context, id uint) (string, error) {
	music, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return "", err
	}
	if music.Img == "" {
		return "", ErrMediaNotFound
	}

	return secureManagedMediaPathAt(s.serverConfig.UploadDir, music.Img)
}

// toEnrichedResponses 批量转换并填充音乐响应列表。
// 统一 Search / ListByUserID / ListLikedByUserID 的「ToResponse + enrich」循环，避免重复逻辑。
func (s *musicService) toEnrichedResponses(ctx context.Context, musics []*domain.Music, currentUserID *uint) ([]*domain.MusicResponse, error) {
	ids := make([]uint, 0, len(musics))
	for _, music := range musics {
		ids = append(ids, music.ID)
	}
	engagement, err := s.repo.ListEngagementByMusicIDs(ctx, ids, currentUserID)
	if err != nil {
		return nil, err
	}
	classifications := make(map[uint]*domain.MusicPresetClassification)
	if s.presetRepo != nil {
		classifications, err = s.presetRepo.FindByMusicIDs(ctx, ids)
		if err != nil {
			return nil, err
		}
	}
	analysisJobs := make(map[uint]*domain.MusicAnalysisJob)
	if s.analysisRepo != nil {
		analysisJobs, err = s.analysisRepo.LatestAudioJobsByMusicIDs(ctx, ids)
		if err != nil {
			return nil, err
		}
	}
	responses := make([]*domain.MusicResponse, 0, len(musics))
	for _, m := range musics {
		resp := s.toResponse(m)
		values := engagement[m.ID]
		resp.LikeCount = values.LikeCount
		resp.IsLiked = values.IsLiked
		resp.PresetClassification = classifications[m.ID].ToResponse()
		resp.AudioAnalysis = analysisJobs[m.ID].ToSummary()
		responses = append(responses, resp)
	}
	return responses, nil
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
	if s.pathResolver == nil {
		return nil
	}
	readOnly, err := s.pathResolver.HasReadOnlyMediaSource(ctx, music.ID)
	if err != nil {
		return err
	}
	if readOnly {
		return ErrForbidden
	}
	return nil
}

// enrichMusicResponse populates one detail response. List responses use the
// repository's batch path to keep playlist and library reads bounded.
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
	if s.presetRepo != nil {
		classifications, err := s.presetRepo.FindByMusicIDs(ctx, []uint{resp.ID})
		if err != nil {
			return err
		}
		resp.PresetClassification = classifications[resp.ID].ToResponse()
	}
	if s.analysisRepo != nil {
		jobs, err := s.analysisRepo.LatestAudioJobsByMusicIDs(ctx, []uint{resp.ID})
		if err != nil {
			return err
		}
		resp.AudioAnalysis = jobs[resp.ID].ToSummary()
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
	if err := s.guardReadOnlyMediaSource(ctx, music, role); err != nil {
		return nil, err
	}
	if err := validateUploadedAudioFileWithLimit(audioHeader, s.uploadPolicy.MaxAudioSizeBytes); err != nil {
		return nil, err
	}
	if err := validateUploadedCoverFileWithLimit(coverHeader, s.uploadPolicy.MaxCoverSizeBytes); err != nil {
		return nil, err
	}

	uploadDir := s.serverConfig.UploadDir
	musicDir := filepath.Join(uploadDir, strconv.FormatUint(uint64(id), 10))
	if err := os.MkdirAll(musicDir, 0700); err != nil {
		return nil, fmt.Errorf("failed to create upload directory: %w", err)
	}

	var staged []*stagedMediaFile
	cleanupStaged := func() {
		for _, file := range staged {
			file.cleanupTemp()
		}
	}
	rollbackApplied := func(cause error) error {
		var rollbackErr error
		for i := len(staged) - 1; i >= 0; i-- {
			if err := staged[i].rollback(); err != nil {
				rollbackErr = errors.Join(rollbackErr, err)
			}
		}
		if rollbackErr != nil {
			return errors.Join(cause, fmt.Errorf("failed to restore previous media files: %w", rollbackErr))
		}
		return cause
	}

	if audioHeader != nil {
		previousPath := music.Path
		tmpPath, finalPath, fileHash, err := saveUploadedMediaFile(musicDir, "audio", audioHeader, true)
		if err != nil {
			cleanupStaged()
			return nil, err
		}
		staged = append(staged, &stagedMediaFile{
			tmpPath:         tmpPath,
			finalPath:       finalPath,
			previousPath:    previousPath,
			cleanupPrevious: pathIsInManagedUploadDirAt(uploadDir, previousPath),
		})
		music.Path = finalPath
		music.FileHash = fileHash
	}

	if coverHeader != nil {
		previousPath := music.Img
		tmpPath, finalPath, _, err := saveUploadedMediaFile(musicDir, "cover", coverHeader, false)
		if err != nil {
			cleanupStaged()
			return nil, err
		}
		staged = append(staged, &stagedMediaFile{
			tmpPath:         tmpPath,
			finalPath:       finalPath,
			previousPath:    previousPath,
			cleanupPrevious: pathIsInManagedUploadDirAt(uploadDir, previousPath),
		})
		music.Img = finalPath
	}

	for _, file := range staged {
		if err := file.apply(); err != nil {
			return nil, rollbackApplied(err)
		}
	}
	if audioHeader != nil {
		info, err := os.Stat(music.Path)
		if err != nil {
			return nil, rollbackApplied(fmt.Errorf("failed to inspect finalized audio file: %w", err))
		}
		relative, err := filepath.Rel(uploadDir, music.Path)
		if err != nil || relative == "." || filepath.IsAbs(relative) || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
			return nil, rollbackApplied(ErrMediaNotFound)
		}
		relative = filepath.ToSlash(relative)
		sourceKey := mediaSourceKey(domain.ManagedMediaRootID, relative, domain.MediaPathSemanticsAuto)
		modifiedAt := info.ModTime().UTC()
		music.MediaRootID = domain.ManagedMediaRootID
		music.MediaRelativePath = relative
		music.MediaSourceKey = &sourceKey
		music.SourceFileSize = info.Size()
		music.SourceFileModTime = &modifiedAt
		music.SourceReadOnly = false
	}

	var persistErr error
	if audioHeader != nil {
		if persister, ok := s.pathResolver.(ManagedMediaSourcePersister); ok {
			persistErr = persister.PersistManagedMusicSource(ctx, music)
		} else {
			persistErr = s.repo.Update(ctx, music)
		}
	} else {
		persistErr = s.repo.Update(ctx, music)
	}
	if persistErr != nil {
		return nil, rollbackApplied(persistErr)
	}

	for _, file := range staged {
		file.commit()
	}
	if audioHeader != nil && s.analyzer != nil {
		scheduleCtx, cancelSchedule := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		if err := s.analyzer.ScheduleContentAnalysis(scheduleCtx, music.ID, userID); err != nil {
			// Analysis is derived work. A full queue or unavailable analyzer must
			// never turn a successfully committed upload into an HTTP failure.
			pklog.Warnf("Music %d was uploaded but analysis could not be queued: %v", music.ID, err)
		}
		cancelSchedule()
	}

	return s.toResponse(music), nil
}

func secureManagedMediaPathAt(uploadDir, path string) (string, error) {
	if strings.TrimSpace(uploadDir) == "" {
		return "", ErrMediaNotFound
	}
	uploadRoot, err := filepath.Abs(filepath.Clean(uploadDir))
	if err != nil {
		return "", ErrMediaNotFound
	}
	absPath, err := filepath.Abs(filepath.Clean(path))
	if err != nil || !pathContains(uploadRoot, absPath) {
		return "", ErrMediaNotFound
	}
	relative, err := filepath.Rel(uploadRoot, absPath)
	if err != nil {
		return "", ErrMediaNotFound
	}
	return securePathWithinRoot(uploadRoot, filepath.ToSlash(relative))
}

func (s *musicService) toResponse(music *domain.Music) *domain.MusicResponse {
	return s.presenter.music(music)
}

type stagedMediaFile struct {
	tmpPath         string
	finalPath       string
	previousPath    string
	backupPath      string
	applied         bool
	cleanupPrevious bool
}

func (f *stagedMediaFile) apply() error {
	if _, err := os.Stat(f.finalPath); err == nil {
		f.backupPath = f.tmpPath + ".backup"
		if err := os.Rename(f.finalPath, f.backupPath); err != nil {
			return fmt.Errorf("failed to back up existing media file: %w", err)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("failed to inspect existing media file: %w", err)
	}

	if err := os.Rename(f.tmpPath, f.finalPath); err != nil {
		if f.backupPath != "" {
			if restoreErr := os.Rename(f.backupPath, f.finalPath); restoreErr != nil {
				return errors.Join(
					fmt.Errorf("failed to finalize media file: %w", err),
					fmt.Errorf("failed to restore media backup: %w", restoreErr),
				)
			}
			f.backupPath = ""
		}
		return fmt.Errorf("failed to finalize media file: %w", err)
	}

	f.applied = true
	return nil
}

func (f *stagedMediaFile) rollback() error {
	var rollbackErr error
	if f.applied {
		if err := os.Remove(f.finalPath); err != nil && !errors.Is(err, os.ErrNotExist) {
			rollbackErr = errors.Join(rollbackErr, fmt.Errorf("remove replacement %s: %w", f.finalPath, err))
		}
	}
	if f.backupPath != "" {
		if err := os.Rename(f.backupPath, f.finalPath); err != nil {
			rollbackErr = errors.Join(rollbackErr, fmt.Errorf("restore backup %s: %w", f.finalPath, err))
		}
	}
	f.cleanupTemp()
	return rollbackErr
}

func (f *stagedMediaFile) commit() {
	cleanupUploadedFiles([]string{f.backupPath})
	if f.cleanupPrevious && f.previousPath != f.finalPath {
		cleanupUploadedFiles([]string{f.previousPath})
	}
}

func pathIsInManagedUploadDirAt(uploadDir, path string) bool {
	if strings.TrimSpace(path) == "" || strings.TrimSpace(uploadDir) == "" {
		return false
	}
	root, err := filepath.Abs(filepath.Clean(uploadDir))
	if err != nil {
		return false
	}
	candidate, err := filepath.Abs(filepath.Clean(path))
	return err == nil && pathContains(root, candidate)
}

func (f *stagedMediaFile) cleanupTemp() {
	cleanupUploadedFiles([]string{f.tmpPath})
}

// saveUploadedMediaFile saves the upload to a unique temporary file. The caller
// promotes it only after every requested media file has been validated and staged.
func saveUploadedMediaFile(dir, baseName string, header *multipart.FileHeader, calculateHash bool) (tmpPath, finalPath string, fileHash string, err error) {
	ext := filepath.Ext(header.Filename)
	finalPath = filepath.Join(dir, baseName+ext)

	src, err := header.Open()
	if err != nil {
		return "", "", "", fmt.Errorf("failed to open %s file: %w", baseName, err)
	}
	defer func() {
		if cerr := src.Close(); cerr != nil && !errors.Is(cerr, io.EOF) && !errors.Is(cerr, io.ErrUnexpectedEOF) {
			pklog.Errorf("Failed to close %s source: %v", baseName, cerr)
		}
	}()

	dst, err := os.CreateTemp(dir, "."+baseName+"-*.tmp")
	if err != nil {
		return "", "", "", fmt.Errorf("failed to create %s file: %w", baseName, err)
	}
	tmpPath = dst.Name()
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
	if artistListCanBeEnriched(existing.Artist, existing.Artists, incoming.Artists) {
		patch.Artists = stringListPointer(incoming.Artists)
		changed = true
	}
	if existing.Album == "" && incoming.Album != "" {
		patch.Album = &incoming.Album
		changed = true
	}
	if existing.AlbumArtist == "" && incoming.AlbumArtist != "" {
		patch.AlbumArtist = &incoming.AlbumArtist
		changed = true
	}
	if artistListCanBeEnriched(existing.AlbumArtist, existing.AlbumArtists, incoming.AlbumArtists) {
		patch.AlbumArtists = stringListPointer(incoming.AlbumArtists)
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
	if existing.TrackTotal == 0 && incoming.TrackTotal > 0 {
		patch.TrackTotal = &incoming.TrackTotal
		changed = true
	}
	if existing.DiscNumber == 0 && incoming.DiscNumber > 0 {
		patch.DiscNumber = &incoming.DiscNumber
		changed = true
	}
	if existing.DiscTotal == 0 && incoming.DiscTotal > 0 {
		patch.DiscTotal = &incoming.DiscTotal
		changed = true
	}
	if existing.ReleaseDate == "" && incoming.ReleaseDate != "" {
		patch.ReleaseDate = &incoming.ReleaseDate
		changed = true
	}
	if existing.OriginalReleaseDate == "" && incoming.OriginalReleaseDate != "" {
		patch.OriginalReleaseDate = &incoming.OriginalReleaseDate
		changed = true
	}
	if existing.Genre == "" && len(existing.Genres) == 0 && len(incoming.Genres) > 0 {
		patch.Genres = stringListPointer(incoming.Genres)
		changed = true
	}
	if existing.Comment == "" && incoming.Comment != "" {
		patch.Comment = &incoming.Comment
		changed = true
	}
	if len(existing.ISRCs) == 0 && len(incoming.ISRCs) > 0 {
		patch.ISRCs = stringListPointer(incoming.ISRCs)
		changed = true
	}
	if existing.Duration == 0 && incoming.Duration > 0 {
		patch.Duration = &incoming.Duration
		changed = true
	}
	if existing.MusicBrainzRecordingID == "" && incoming.MusicBrainzRecordingID != "" {
		patch.MusicBrainzRecordingID = &incoming.MusicBrainzRecordingID
		changed = true
	}
	if existing.MusicBrainzTrackID == "" && incoming.MusicBrainzTrackID != "" {
		patch.MusicBrainzTrackID = &incoming.MusicBrainzTrackID
		changed = true
	}
	if existing.MusicBrainzReleaseID == "" && incoming.MusicBrainzReleaseID != "" {
		patch.MusicBrainzReleaseID = &incoming.MusicBrainzReleaseID
		changed = true
	}
	if existing.MusicBrainzReleaseGroupID == "" && incoming.MusicBrainzReleaseGroupID != "" {
		patch.MusicBrainzReleaseGroupID = &incoming.MusicBrainzReleaseGroupID
		changed = true
	}
	if len(existing.MusicBrainzArtistIDs) == 0 && len(incoming.MusicBrainzArtistIDs) > 0 {
		patch.MusicBrainzArtistIDs = stringListPointer(incoming.MusicBrainzArtistIDs)
		changed = true
	}
	if len(existing.MusicBrainzAlbumArtistIDs) == 0 && len(incoming.MusicBrainzAlbumArtistIDs) > 0 {
		patch.MusicBrainzAlbumArtistIDs = stringListPointer(incoming.MusicBrainzAlbumArtistIDs)
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
	if len(incoming.Artists) == 0 ||
		(len(incoming.Artists) == 1 && strings.EqualFold(incoming.Artists[0], incoming.Artist) && len(best.Artists) > 1) {
		incoming.Artists = append(domain.StringList{}, best.Artists...)
	}
	if incoming.AlbumArtist == "" {
		incoming.AlbumArtist = best.AlbumArtist
	}
	if len(incoming.AlbumArtists) == 0 ||
		(len(incoming.AlbumArtists) == 1 && strings.EqualFold(incoming.AlbumArtists[0], incoming.AlbumArtist) && len(best.AlbumArtists) > 1) {
		incoming.AlbumArtists = append(domain.StringList{}, best.AlbumArtists...)
	}
	if incoming.Year == 0 {
		incoming.Year = best.Year
	}
	if incoming.TrackNumber == 0 {
		incoming.TrackNumber = best.TrackNumber
	}
	if incoming.TrackTotal == 0 {
		incoming.TrackTotal = best.TrackTotal
	}
	if incoming.DiscNumber == 0 {
		incoming.DiscNumber = best.DiscNumber
	}
	if incoming.DiscTotal == 0 {
		incoming.DiscTotal = best.DiscTotal
	}
	if incoming.ReleaseDate == "" {
		incoming.ReleaseDate = best.ReleaseDate
	}
	if incoming.OriginalReleaseDate == "" {
		incoming.OriginalReleaseDate = best.OriginalReleaseDate
	}
	if incoming.Genre == "" && len(incoming.Genres) == 0 {
		incoming.Genre = best.Genre
		incoming.Genres = append(domain.StringList{}, best.Genres...)
	}
	if incoming.Comment == "" {
		incoming.Comment = best.Comment
	}
	if len(incoming.ISRCs) == 0 {
		incoming.ISRCs = append(domain.StringList{}, best.ISRCs...)
	}
	if incoming.Duration == 0 {
		incoming.Duration = best.Duration
	}
	if incoming.MusicBrainzRecordingID == "" {
		incoming.MusicBrainzRecordingID = best.MusicBrainzRecordingID
	}
	if incoming.MusicBrainzTrackID == "" {
		incoming.MusicBrainzTrackID = best.MusicBrainzTrackID
	}
	if incoming.MusicBrainzReleaseID == "" {
		incoming.MusicBrainzReleaseID = best.MusicBrainzReleaseID
	}
	if incoming.MusicBrainzReleaseGroupID == "" {
		incoming.MusicBrainzReleaseGroupID = best.MusicBrainzReleaseGroupID
	}
	if len(incoming.MusicBrainzArtistIDs) == 0 {
		incoming.MusicBrainzArtistIDs = append(domain.StringList{}, best.MusicBrainzArtistIDs...)
	}
	if len(incoming.MusicBrainzAlbumArtistIDs) == 0 {
		incoming.MusicBrainzAlbumArtistIDs = append(domain.StringList{}, best.MusicBrainzAlbumArtistIDs...)
	}
	return incoming
}

func metadataCompleteness(music *domain.Music) int {
	score := 0
	if music.Album != "" {
		score++
	}
	if len(music.Artists) > 1 {
		score++
	}
	if music.AlbumArtist != "" || len(music.AlbumArtists) > 0 {
		score++
	}
	if music.Year > 0 {
		score++
	}
	if music.TrackNumber > 0 {
		score++
	}
	if music.TrackTotal > 0 {
		score++
	}
	if music.DiscNumber > 0 || music.DiscTotal > 0 {
		score++
	}
	if music.ReleaseDate != "" || music.OriginalReleaseDate != "" {
		score++
	}
	if music.Genre != "" || len(music.Genres) > 0 {
		score++
	}
	if music.Comment != "" || len(music.ISRCs) > 0 {
		score++
	}
	if music.Duration > 0 {
		score++
	}
	if music.MusicBrainzRecordingID != "" || music.MusicBrainzTrackID != "" {
		score += 2
	}
	if music.MusicBrainzReleaseID != "" || music.MusicBrainzReleaseGroupID != "" {
		score++
	}
	if len(music.MusicBrainzArtistIDs) > 0 || len(music.MusicBrainzAlbumArtistIDs) > 0 {
		score++
	}
	return score
}

func normalizedMusicMetadata(metadata domain.MusicMetadata) (domain.MusicMetadata, error) {
	music := &domain.Music{}
	request := &domain.CreateMusicRequest{
		Title:                     metadata.Title,
		Artist:                    metadata.Artist,
		Artists:                   metadata.Artists,
		Album:                     metadata.Album,
		AlbumArtist:               metadata.AlbumArtist,
		AlbumArtists:              metadata.AlbumArtists,
		Year:                      metadata.Year,
		TrackNumber:               metadata.TrackNumber,
		TrackTotal:                metadata.TrackTotal,
		DiscNumber:                metadata.DiscNumber,
		DiscTotal:                 metadata.DiscTotal,
		ReleaseDate:               metadata.ReleaseDate,
		OriginalReleaseDate:       metadata.OriginalReleaseDate,
		Genre:                     metadata.Genre,
		Genres:                    metadata.Genres,
		Comment:                   metadata.Comment,
		ISRCs:                     metadata.ISRCs,
		Duration:                  metadata.Duration,
		MusicBrainzRecordingID:    metadata.MusicBrainzRecordingID,
		MusicBrainzTrackID:        metadata.MusicBrainzTrackID,
		MusicBrainzReleaseID:      metadata.MusicBrainzReleaseID,
		MusicBrainzReleaseGroupID: metadata.MusicBrainzReleaseGroupID,
		MusicBrainzArtistIDs:      metadata.MusicBrainzArtistIDs,
		MusicBrainzAlbumArtistIDs: metadata.MusicBrainzAlbumArtistIDs,
	}
	if err := applyCreateMusicMetadata(music, request); err != nil {
		return domain.MusicMetadata{}, err
	}
	return musicMetadataFromMusic(music), nil
}

func artistListCanBeEnriched(credited string, existing, incoming domain.StringList) bool {
	if len(incoming) == 0 {
		return false
	}
	if len(existing) == 0 {
		return true
	}
	return len(existing) == 1 && strings.EqualFold(existing[0], credited) && len(incoming) > 1
}

func stringListPointer(values domain.StringList) *domain.StringList {
	copy := append(domain.StringList{}, values...)
	return &copy
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
