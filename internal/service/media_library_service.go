package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/kyangconn/music-online-go/internal/config"
	"github.com/kyangconn/music-online-go/internal/domain"
	"github.com/kyangconn/music-online-go/internal/mediafs"
	pklog "github.com/kyangconn/music-online-go/internal/pkg/log"
	"github.com/kyangconn/music-online-go/internal/repository"
)

var (
	ErrLibraryScannerDisabled  = errors.New("server media library scanner is disabled")
	ErrInvalidMediaRoot        = errors.New("invalid media library root")
	ErrMediaRootOverlap        = errors.New("media library roots cannot overlap")
	ErrMediaRootPathLocked     = errors.New("media library root path cannot change after tracks have been indexed")
	ErrMediaStorageUnavailable = errors.New("media storage is temporarily unavailable")
)

func IsTransientMediaStorageError(err error) bool {
	return mediafs.IsTransientError(err)
}

func isRetryableMediaStorageError(err error) bool {
	return mediafs.IsRetryableError(err)
}

const (
	mediaScanPollInterval = time.Second
	// Lease timing is an internal consistency mechanism, not a deployment
	// tuning knob. Long file hashes refresh it periodically.
	mediaScanLeaseDuration = 15 * time.Minute
	mediaScanIssueLimit    = 200
	unknownArtist          = "Unknown Artist"
)

type MediaPathResolver interface {
	ResolveMusicPath(ctx context.Context, music *domain.Music) (string, error)
	HasReadOnlyMediaSource(ctx context.Context, musicID uint) (bool, error)
}

type ManagedMediaSourcePersister interface {
	PersistManagedMusicSource(ctx context.Context, music *domain.Music) error
}

type MediaLibraryService interface {
	MediaPathResolver
	ManagedMediaSourcePersister
	ListRoots(ctx context.Context) ([]*domain.MediaLibraryRootResponse, error)
	CreateRoot(ctx context.Context, userID uint, req *domain.CreateMediaLibraryRootRequest) (*domain.MediaLibraryRootResponse, error)
	UpdateRoot(ctx context.Context, id uint, req *domain.UpdateMediaLibraryRootRequest) (*domain.MediaLibraryRootResponse, error)
	DeleteRoot(ctx context.Context, id uint) error
	StartScan(ctx context.Context, rootID, userID uint) (*domain.MediaScanJob, error)
	ListScanJobs(ctx context.Context, rootID *uint, page, pageSize int) ([]*domain.MediaScanJob, int64, error)
	GetScanJob(ctx context.Context, id uint) (*domain.MediaScanJobDetail, error)
	CancelScan(ctx context.Context, id uint) (*domain.MediaScanJob, error)
	ProbeRoot(ctx context.Context, id uint) (*domain.MediaLibraryRootHealthResponse, error)
	Start(ctx context.Context) error
	Shutdown(ctx context.Context) error
}

type mediaLibraryService struct {
	repo          repository.MediaLibraryRepository
	musicRepo     repository.MusicRepository
	config        config.LibraryConfig
	prober        mediafs.Prober
	managedPath   string
	maxCoverBytes int64
	workerID      string

	rootMu  sync.Mutex
	mu      sync.Mutex
	cancel  context.CancelFunc
	started bool
	wg      sync.WaitGroup
}

type resolvedMediaRoot struct {
	ID                 uint
	Key                string
	Name               string
	Path               string
	StorageKind        string
	ExpectedFilesystem string
	ProbeFile          string
	PathSemantics      string
	Enabled            bool
	ReadOnly           bool
}

func NewMediaLibraryService(repo repository.MediaLibraryRepository, musicRepo repository.MusicRepository, cfg config.LibraryConfig, serverConfigs ...config.ServerConfig) MediaLibraryService {
	serverConfig := config.ServerConfig{UploadDir: "uploads", MaxCoverSizeMB: config.DefaultMaxCoverSizeMB}
	if len(serverConfigs) > 0 {
		serverConfig = serverConfigs[0]
	} else if config.AppConfig != nil {
		serverConfig = config.AppConfig.Server
	}
	return NewMediaLibraryServiceWithProber(repo, musicRepo, cfg, serverConfig, mediafs.NewSystemProber())
}

// NewMediaLibraryServiceWithProber keeps platform I/O behind a small boundary;
// deterministic tests can model NFS/SMB failures without requiring those mounts.
func NewMediaLibraryServiceWithProber(repo repository.MediaLibraryRepository, musicRepo repository.MusicRepository, cfg config.LibraryConfig, serverConfig config.ServerConfig, prober mediafs.Prober) MediaLibraryService {
	managedPath := strings.TrimSpace(serverConfig.UploadDir)
	if managedPath == "" {
		managedPath = "uploads"
	}
	if prober == nil {
		prober = mediafs.NewSystemProber()
	}
	return &mediaLibraryService{
		repo:          repo,
		musicRepo:     musicRepo,
		config:        cfg,
		prober:        prober,
		managedPath:   managedPath,
		maxCoverBytes: int64(serverConfig.MaxCoverSizeMB) * 1024 * 1024,
		workerID:      newMediaWorkerID(),
	}
}

func (s *mediaLibraryService) ListRoots(ctx context.Context) ([]*domain.MediaLibraryRootResponse, error) {
	managed, err := s.resolveRoot(ctx, domain.ManagedMediaRootID)
	if err != nil {
		return nil, err
	}
	managedResponse, err := s.mediaRootResponse(ctx, managed, time.Time{}, time.Time{}, 0)
	if err != nil {
		return nil, err
	}
	responses := []*domain.MediaLibraryRootResponse{managedResponse}
	roots, err := s.repo.ListRoots(ctx)
	if err != nil {
		return nil, err
	}
	for _, root := range roots {
		response, err := s.mediaRootResponse(ctx, resolvedRootFromDomain(root), root.CreatedAt, root.UpdatedAt, root.CreatedBy)
		if err != nil {
			return nil, err
		}
		responses = append(responses, response)
	}
	return responses, nil
}

func (s *mediaLibraryService) CreateRoot(ctx context.Context, userID uint, req *domain.CreateMediaLibraryRootRequest) (*domain.MediaLibraryRootResponse, error) {
	s.rootMu.Lock()
	defer s.rootMu.Unlock()
	name := strings.TrimSpace(req.Name)
	path, err := normalizeMediaRootPath(req.Path)
	if err != nil {
		return nil, err
	}
	if name == "" || utf8.RuneCountInString(name) > 100 {
		return nil, fmt.Errorf("%w: name must contain 1 to 100 characters", ErrInvalidMediaRoot)
	}
	if err := s.ensureRootDoesNotOverlap(ctx, 0, path); err != nil {
		return nil, err
	}
	storageKind := normalizeStorageKind(req.StorageKind)
	pathSemantics := normalizeMediaPathSemantics(req.PathSemantics)
	expectedFilesystem := strings.ToLower(strings.TrimSpace(req.ExpectedFilesystem))
	probeFile := strings.TrimSpace(req.ProbeFile)
	if err := validateResolvedRootSpec(&resolvedMediaRoot{
		Path: path, StorageKind: storageKind, ExpectedFilesystem: expectedFilesystem,
		ProbeFile: probeFile, PathSemantics: pathSemantics,
	}); err != nil {
		return nil, err
	}
	root := &domain.MediaLibraryRoot{
		Key: newMediaRootKey(), Name: name, Path: path, StorageKind: storageKind,
		ExpectedFilesystem: expectedFilesystem, ProbeFile: probeFile, PathSemantics: pathSemantics,
		Enabled: true, ReadOnly: true, CreatedBy: userID,
	}
	if err := s.repo.CreateRoot(ctx, root); err != nil {
		return nil, err
	}
	return s.mediaRootResponse(ctx, resolvedRootFromDomain(root), root.CreatedAt, root.UpdatedAt, root.CreatedBy)
}

func (s *mediaLibraryService) UpdateRoot(ctx context.Context, id uint, req *domain.UpdateMediaLibraryRootRequest) (*domain.MediaLibraryRootResponse, error) {
	s.rootMu.Lock()
	defer s.rootMu.Unlock()
	if id == domain.ManagedMediaRootID {
		return nil, fmt.Errorf("%w: the managed root is changed through server.upload_dir", ErrInvalidMediaRoot)
	}
	root, err := s.repo.FindRootByID(ctx, id)
	if err != nil {
		return nil, err
	}
	active, err := s.repo.HasActiveScanForRoot(ctx, id)
	if err != nil {
		return nil, err
	}
	if active {
		return nil, repository.ErrMediaScanInProgress
	}
	if req.Name != nil {
		root.Name = strings.TrimSpace(*req.Name)
		if root.Name == "" || utf8.RuneCountInString(root.Name) > 100 {
			return nil, fmt.Errorf("%w: name must contain 1 to 100 characters", ErrInvalidMediaRoot)
		}
	}
	pathOrSemanticsChanged := false
	if req.Path != nil {
		path, err := normalizeMediaRootPath(*req.Path)
		if err != nil {
			return nil, err
		}
		if !sameFilesystemPath(root.Path, path) {
			if err := s.ensureRootDoesNotOverlap(ctx, id, path); err != nil {
				return nil, err
			}
			root.Path = path
			pathOrSemanticsChanged = true
		}
	}
	if req.StorageKind != nil {
		root.StorageKind = normalizeStorageKind(*req.StorageKind)
	}
	if req.ExpectedFilesystem != nil {
		root.ExpectedFilesystem = strings.ToLower(strings.TrimSpace(*req.ExpectedFilesystem))
	}
	if req.ProbeFile != nil {
		root.ProbeFile = strings.TrimSpace(*req.ProbeFile)
	}
	if req.PathSemantics != nil {
		semantics := normalizeMediaPathSemantics(*req.PathSemantics)
		if semantics != normalizeMediaPathSemantics(root.PathSemantics) {
			root.PathSemantics = semantics
			pathOrSemanticsChanged = true
		}
	}
	if pathOrSemanticsChanged {
		count, err := s.repo.CountMusicByRoot(ctx, id)
		if err != nil {
			return nil, err
		}
		if count > 0 {
			return nil, ErrMediaRootPathLocked
		}
	}
	if req.Enabled != nil {
		root.Enabled = *req.Enabled
	}
	root.ReadOnly = true
	if err := validateResolvedRootSpec(resolvedRootFromDomain(root)); err != nil {
		return nil, err
	}
	if err := s.repo.UpdateRoot(ctx, root); err != nil {
		return nil, err
	}
	checkedAt := time.Now().UTC()
	previousState, _ := s.repo.FindRootState(ctx, root.ID)
	unknownState := &domain.MediaLibraryRootState{
		RootID: root.ID, Status: domain.MediaRootHealthUnknown, Code: "configuration_changed",
		Message: "storage configuration changed and has not been checked yet", LastCheckedAt: &checkedAt,
	}
	if previousState != nil {
		unknownState.LastOnlineAt = previousState.LastOnlineAt
	}
	if err := s.repo.UpsertRootState(ctx, unknownState); err != nil {
		pklog.Errorf("Failed to reset health for media root %d: %v", root.ID, err)
	}
	return s.mediaRootResponse(ctx, resolvedRootFromDomain(root), root.CreatedAt, root.UpdatedAt, root.CreatedBy)
}

func (s *mediaLibraryService) DeleteRoot(ctx context.Context, id uint) error {
	s.rootMu.Lock()
	defer s.rootMu.Unlock()
	if id == domain.ManagedMediaRootID {
		return fmt.Errorf("%w: the managed root cannot be deleted", ErrInvalidMediaRoot)
	}
	return s.repo.DeleteRoot(ctx, id)
}

func (s *mediaLibraryService) StartScan(ctx context.Context, rootID, userID uint) (*domain.MediaScanJob, error) {
	s.rootMu.Lock()
	defer s.rootMu.Unlock()
	if !s.config.Scanner.Enabled {
		return nil, ErrLibraryScannerDisabled
	}
	root, err := s.resolveRoot(ctx, rootID)
	if err != nil {
		return nil, err
	}
	if !root.Enabled {
		return nil, fmt.Errorf("%w: media library root is disabled", ErrInvalidMediaRoot)
	}
	job := &domain.MediaScanJob{
		RootID: root.ID, RootName: root.Name, RequestedBy: userID, Status: domain.MediaScanStatusPending,
	}
	if err := s.repo.CreateScanJob(ctx, job); err != nil {
		return nil, err
	}
	return job, nil
}

func (s *mediaLibraryService) ListScanJobs(ctx context.Context, rootID *uint, page, pageSize int) ([]*domain.MediaScanJob, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	return s.repo.ListScanJobs(ctx, rootID, page, pageSize)
}

func (s *mediaLibraryService) GetScanJob(ctx context.Context, id uint) (*domain.MediaScanJobDetail, error) {
	job, err := s.repo.FindScanJob(ctx, id)
	if err != nil {
		return nil, err
	}
	issues, err := s.repo.ListScanIssues(ctx, id, mediaScanIssueLimit)
	if err != nil {
		return nil, err
	}
	return &domain.MediaScanJobDetail{MediaScanJob: *job, Issues: issues}, nil
}

func (s *mediaLibraryService) CancelScan(ctx context.Context, id uint) (*domain.MediaScanJob, error) {
	return s.repo.RequestScanCancellation(ctx, id)
}

func (s *mediaLibraryService) ProbeRoot(ctx context.Context, id uint) (*domain.MediaLibraryRootHealthResponse, error) {
	root, err := s.resolveRoot(ctx, id)
	if err != nil {
		return nil, err
	}
	state, _ := s.probeResolvedRoot(ctx, root)
	health := mediaRootHealthResponse(state)
	return &health, nil
}

func (s *mediaLibraryService) Start(parent context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.started {
		return nil
	}
	if err := s.repo.RecoverExpiredScanJobs(parent); err != nil {
		return fmt.Errorf("recover interrupted media scans: %w", err)
	}
	s.started = true
	ctx, cancel := context.WithCancel(parent)
	s.cancel = cancel
	if s.config.Scanner.Enabled {
		s.wg.Add(1)
		go s.scanWorker(ctx)
	}
	if s.config.HealthCheckIntervalSeconds > 0 {
		s.wg.Add(1)
		go s.healthWorker(ctx)
	}
	return nil
}

func (s *mediaLibraryService) Shutdown(ctx context.Context) error {
	s.mu.Lock()
	if s.cancel != nil {
		s.cancel()
	}
	s.mu.Unlock()
	done := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (s *mediaLibraryService) ResolveMusicPath(ctx context.Context, music *domain.Music) (string, error) {
	if music == nil {
		return "", ErrMediaNotFound
	}
	mediaFiles, err := s.repo.ListMediaFilesByMusicID(ctx, music.ID)
	if err != nil {
		return "", err
	}
	// Keep old databases and in-flight uploads playable while migration 5
	// gradually makes MediaFile the authoritative physical-source record.
	if len(mediaFiles) == 0 && strings.TrimSpace(music.MediaRelativePath) != "" {
		mediaFiles = []*domain.MediaFile{{
			MusicID: music.ID, RootID: music.MediaRootID, RelativePath: music.MediaRelativePath,
			Availability: domain.MediaFileAvailabilityUnknown,
		}}
	}
	hadUnavailableStorage := false
	for _, mediaFile := range mediaFiles {
		if mediaFile.Availability == domain.MediaFileAvailabilityChanged || mediaFile.Availability == domain.MediaFileAvailabilityMissing {
			continue
		}
		root, rootErr := s.resolveRoot(ctx, mediaFile.RootID)
		if rootErr != nil || !root.Enabled {
			continue
		}
		state, stateErr := s.repo.FindRootState(ctx, root.ID)
		if stateErr != nil {
			return "", stateErr
		}
		if state != nil && state.Status == domain.MediaRootHealthOffline {
			interval := time.Duration(s.config.HealthCheckIntervalSeconds) * time.Second
			if interval <= 0 {
				interval = time.Minute
			}
			if state.LastCheckedAt == nil || time.Since(*state.LastCheckedAt) >= interval {
				state, _ = s.probeResolvedRoot(ctx, root)
			}
			if state.Status == domain.MediaRootHealthOffline {
				hadUnavailableStorage = true
				continue
			}
		}
		path, pathErr := securePathWithinRoot(root.Path, mediaFile.RelativePath)
		if pathErr != nil {
			probed, result := s.probeResolvedRoot(ctx, root)
			if !result.Available() {
				hadUnavailableStorage = true
				mediaFile.Availability = domain.MediaFileAvailabilityOffline
			} else {
				mediaFile.Availability = domain.MediaFileAvailabilityMissing
			}
			if mediaFile.ID != 0 {
				mediaFile.LastVerifiedAt = probed.LastCheckedAt
				_ = s.repo.UpdateMediaFile(ctx, mediaFile)
			}
			continue
		}
		if mediaFile.ID != 0 && mediaFile.Availability != domain.MediaFileAvailabilityOnline {
			now := time.Now().UTC()
			mediaFile.Availability = domain.MediaFileAvailabilityOnline
			mediaFile.LastVerifiedAt = &now
			_ = s.repo.UpdateMediaFile(ctx, mediaFile)
		}
		return path, nil
	}
	if hadUnavailableStorage {
		return "", ErrMediaStorageUnavailable
	}
	return "", ErrMediaNotFound
}

// HasReadOnlyMediaSource protects a logical track once an administrator has
// linked it to an external library. The decision is based on every physical
// source, not the compatibility fields on Music, so hash-linked duplicates are
// covered as well.
func (s *mediaLibraryService) HasReadOnlyMediaSource(ctx context.Context, musicID uint) (bool, error) {
	return s.repo.HasReadOnlyMediaFile(ctx, musicID)
}

func (s *mediaLibraryService) PersistManagedMusicSource(ctx context.Context, music *domain.Music) error {
	if music == nil || music.ID == 0 || strings.TrimSpace(music.MediaRelativePath) == "" || music.MediaSourceKey == nil {
		return ErrMediaNotFound
	}
	now := time.Now().UTC()
	contentRevision := uint64(1)
	existing, err := s.repo.FindMediaFileBySourceKey(ctx, *music.MediaSourceKey)
	if err != nil && !errors.Is(err, repository.ErrMediaFileNotFound) {
		return err
	}
	if existing != nil {
		contentRevision = existing.ContentRevision
		if contentRevision == 0 {
			contentRevision = 1
		}
		if !strings.EqualFold(existing.FileHash, music.FileHash) {
			contentRevision++
		}
	}
	mediaFile := &domain.MediaFile{
		MusicID:          music.ID,
		RootID:           domain.ManagedMediaRootID,
		RelativePath:     music.MediaRelativePath,
		SourceKey:        *music.MediaSourceKey,
		FileHash:         music.FileHash,
		ObservedFileHash: music.FileHash,
		FileSize:         music.SourceFileSize,
		FileModTime:      music.SourceFileModTime,
		ReadOnly:         false,
		Availability:     domain.MediaFileAvailabilityOnline,
		ContentRevision:  contentRevision,
		LastSeenAt:       &now,
		LastVerifiedAt:   &now,
	}
	return s.repo.ReplaceManagedMediaFile(ctx, music, mediaFile)
}

func (s *mediaLibraryService) scanWorker(ctx context.Context) {
	defer s.wg.Done()
	ticker := time.NewTicker(mediaScanPollInterval)
	defer ticker.Stop()
	lastLeaseRecovery := time.Time{}
	for {
		if lastLeaseRecovery.IsZero() || time.Since(lastLeaseRecovery) >= 30*time.Second {
			if err := s.repo.RecoverExpiredScanJobs(ctx); err != nil && ctx.Err() == nil {
				pklog.Errorf("Failed to recover expired media scan leases: %v", err)
			}
			lastLeaseRecovery = time.Now()
		}
		job, found, err := s.repo.ClaimNextScanJob(ctx, s.workerID, mediaScanLeaseDuration)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			pklog.Errorf("Failed to claim media scan job: %v", err)
		} else if found {
			s.runScanJob(ctx, job)
			continue
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (s *mediaLibraryService) healthWorker(ctx context.Context) {
	defer s.wg.Done()
	interval := time.Duration(s.config.HealthCheckIntervalSeconds) * time.Second
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	s.probeAllRoots(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.probeAllRoots(ctx)
		}
	}
}

func (s *mediaLibraryService) probeAllRoots(ctx context.Context) {
	managed, err := s.resolveRoot(ctx, domain.ManagedMediaRootID)
	if err == nil {
		s.probeResolvedRoot(ctx, managed)
	}
	roots, err := s.repo.ListRoots(ctx)
	if err != nil {
		pklog.Errorf("Failed to list media roots for health check: %v", err)
		return
	}
	for _, root := range roots {
		if ctx.Err() != nil {
			return
		}
		if root.Enabled {
			s.probeResolvedRoot(ctx, resolvedRootFromDomain(root))
		}
	}
}

func (s *mediaLibraryService) runScanJob(workerCtx context.Context, job *domain.MediaScanJob) {
	scanStartedAt := time.Now().UTC()
	root, err := s.resolveRoot(workerCtx, job.RootID)
	if err != nil || !root.Enabled {
		s.finishScanJob(workerCtx, job, domain.MediaScanStatusFailed, "root_unavailable", false, "media library root is missing or disabled")
		return
	}
	state, probeResult := s.probeResolvedRoot(workerCtx, root)
	if !probeResult.Available() {
		s.retryOrFinishScan(workerCtx, job, state.Code, state.Retryable, state.Message)
		return
	}

	cancelledByUser := false
	workerStopped := false
	transientStorageFailure := false
	lastCancellationCheck := time.Time{}
	lastProgressFlush := time.Now()
	walkErr := filepath.WalkDir(root.Path, func(path string, entry fs.DirEntry, walkErr error) error {
		if workerCtx.Err() != nil {
			workerStopped = true
			return fs.SkipAll
		}
		now := time.Now()
		if lastCancellationCheck.IsZero() || now.Sub(lastCancellationCheck) >= time.Second {
			requested, err := s.repo.IsScanCancellationRequested(workerCtx, job.ID)
			if err != nil {
				return err
			}
			lastCancellationCheck = now
			if requested {
				cancelledByUser = true
				return fs.SkipAll
			}
		}
		if walkErr != nil {
			if isRetryableMediaStorageError(walkErr) {
				transientStorageFailure = true
			}
			relative := safeRelativePath(root.Path, path)
			job.FailedCount++
			s.addScanIssue(workerCtx, job, relative, "error", "directory_unavailable", fmt.Sprintf("directory entry could not be read: %v", walkErr))
			if entry != nil && entry.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if entry.IsDir() || entry.Type()&os.ModeSymlink != 0 || !isScannableAudioName(entry.Name()) {
			return nil
		}
		job.DiscoveredCount++
		s.processScanFile(workerCtx, job, root, path, entry, &transientStorageFailure)
		job.ProcessedCount++
		now = time.Now()
		if job.ProcessedCount%25 == 0 || now.Sub(lastProgressFlush) >= 2*time.Second {
			heartbeat := now.UTC()
			job.HeartbeatAt = &heartbeat
			leaseExpiry := heartbeat.Add(mediaScanLeaseDuration)
			job.LeaseExpiresAt = &leaseExpiry
			if err := s.repo.UpdateScanJob(workerCtx, job); err != nil {
				return err
			}
			lastProgressFlush = now
		}
		return nil
	})

	if workerStopped || workerCtx.Err() != nil {
		// Leave the running lease untouched on process shutdown. It will be
		// reclaimed only after expiry, avoiding a second worker racing this one.
		return
	}
	if cancelledByUser {
		s.finishScanJob(context.WithoutCancel(workerCtx), job, domain.MediaScanStatusCancelled, "cancelled", false, "scan cancelled; imported tracks were kept")
		return
	}
	if walkErr != nil {
		if errors.Is(walkErr, repository.ErrMediaScanLeaseLost) {
			return
		}
		state, result := s.probeResolvedRoot(workerCtx, root)
		if !result.Available() {
			s.retryOrFinishScan(workerCtx, job, state.Code, state.Retryable, state.Message)
			return
		}
		if transientStorageFailure {
			s.retryOrFinishScan(workerCtx, job, "storage_read_interrupted", true, "network storage recovered after interrupting the directory walk")
			return
		}
		s.finishScanJob(workerCtx, job, domain.MediaScanStatusFailed, "walk_failed", false, "scan stopped because a directory could not be read")
		return
	}
	if transientStorageFailure {
		state, result := s.probeResolvedRoot(workerCtx, root)
		if !result.Available() {
			s.retryOrFinishScan(workerCtx, job, state.Code, state.Retryable, state.Message)
		} else {
			s.retryOrFinishScan(workerCtx, job, "storage_read_interrupted", true, "network storage recovered after one or more source reads were interrupted")
		}
		return
	}
	if job.FailedCount > 0 {
		state, result := s.probeResolvedRoot(workerCtx, root)
		if !result.Available() {
			s.retryOrFinishScan(workerCtx, job, state.Code, state.Retryable, state.Message)
			return
		}
	}
	summary := ""
	if job.FailedCount > 0 || job.WarningCount > 0 {
		summary = fmt.Sprintf("scan completed with %d error(s) and %d warning(s)", job.FailedCount, job.WarningCount)
	}
	// A successful pass marks vanished sources unavailable but deliberately
	// keeps the logical track and metadata, matching append-oriented libraries.
	if job.FailedCount == 0 {
		if err := s.repo.MarkRootFilesMissingBefore(workerCtx, root.ID, scanStartedAt); err != nil {
			s.finishScanJob(workerCtx, job, domain.MediaScanStatusFailed, "database_write_failed", false, "scan completed but missing source state could not be reconciled")
			return
		}
	}
	s.finishScanJob(workerCtx, job, domain.MediaScanStatusSucceeded, "", false, summary)
}

func (s *mediaLibraryService) processScanFile(ctx context.Context, job *domain.MediaScanJob, root *resolvedMediaRoot, path string, entry fs.DirEntry, transientStorageFailure *bool) {
	relative := safeRelativePath(root.Path, path)
	info, err := entry.Info()
	if err != nil {
		if isRetryableMediaStorageError(err) {
			*transientStorageFailure = true
		}
		job.FailedCount++
		s.addScanIssue(ctx, job, relative, "error", "file_unavailable", "file information could not be read")
		return
	}
	if !info.Mode().IsRegular() {
		job.SkippedCount++
		return
	}
	sourceKey := mediaSourceKey(root.ID, relative, root.PathSemantics)
	existingFile, err := s.repo.FindMediaFileBySourceKey(ctx, sourceKey)
	if err != nil && !errors.Is(err, repository.ErrMediaFileNotFound) {
		job.FailedCount++
		s.addScanIssue(ctx, job, relative, "error", "database_read_failed", "existing track state could not be checked")
		return
	}
	now := time.Now().UTC()
	markExistingSeen := func(availability string) bool {
		if existingFile == nil {
			return true
		}
		existingFile.LastSeenAt = &now
		if availability != "" {
			existingFile.Availability = availability
		}
		if err := s.repo.UpdateMediaFile(ctx, existingFile); err != nil {
			job.FailedCount++
			s.addScanIssue(ctx, job, relative, "error", "database_write_failed", "source file state could not be refreshed")
			return false
		}
		return true
	}
	if info.Size() <= 0 {
		if !markExistingSeen(domain.MediaFileAvailabilityChanged) {
			return
		}
		job.SkippedCount++
		s.addScanIssue(ctx, job, relative, "warning", "empty_file", "empty audio file was skipped")
		return
	}
	maxBytes := int64(s.config.Scanner.MaxFileSizeMB) * 1024 * 1024
	if info.Size() > maxBytes {
		if !markExistingSeen(domain.MediaFileAvailabilityChanged) {
			return
		}
		job.SkippedCount++
		s.addScanIssue(ctx, job, relative, "warning", "file_too_large", "audio file exceeds the configured scanner size limit")
		return
	}
	minimumAge := time.Duration(s.config.Scanner.MinFileAgeSeconds) * time.Second
	if minimumAge > 0 && time.Since(info.ModTime()) < minimumAge {
		if !markExistingSeen("") {
			return
		}
		job.SkippedCount++
		return
	}
	if existingFile != nil && existingFile.FileSize == info.Size() && sameFileTimestamp(existingFile.FileModTime, info.ModTime()) &&
		!sourceNeedsHashRecheck(existingFile.LastVerifiedAt, s.config.Scanner.HashRecheckHours, now) {
		if existingFile.ObservedFileHash == "" {
			existingFile.ObservedFileHash = existingFile.FileHash
		}
		existingFile.Availability = domain.MediaFileAvailabilityOnline
		existingFile.LastSeenAt = &now
		if err := s.repo.UpdateMediaFile(ctx, existingFile); err != nil {
			job.FailedCount++
			s.addScanIssue(ctx, job, relative, "error", "database_write_failed", "source file state could not be refreshed")
			return
		}
		job.ExistingCount++
		return
	}

	if err := validateScannedAudio(path); err != nil {
		if errors.Is(err, ErrInvalidMediaFile) {
			if existingFile != nil {
				existingFile.Availability = domain.MediaFileAvailabilityChanged
				existingFile.LastSeenAt = &now
				_ = s.repo.UpdateMediaFile(ctx, existingFile)
			}
			job.SkippedCount++
			s.addScanIssue(ctx, job, relative, "warning", "unsupported_content", "file does not contain a supported audio signature")
		} else {
			if isRetryableMediaStorageError(err) {
				*transientStorageFailure = true
			}
			if existingFile != nil {
				existingFile.Availability = domain.MediaFileAvailabilityOffline
				_ = s.repo.UpdateMediaFile(ctx, existingFile)
			}
			job.FailedCount++
			s.addScanIssue(ctx, job, relative, "error", "file_unavailable", fmt.Sprintf("audio header could not be read: %v", err))
		}
		return
	}
	fileHash, err := hashScannedFile(ctx, path, func() error {
		return s.renewScanLease(ctx, job)
	})
	if err != nil {
		if errors.Is(err, repository.ErrMediaScanLeaseLost) {
			return
		}
		if existingFile != nil {
			existingFile.Availability = domain.MediaFileAvailabilityOffline
			_ = s.repo.UpdateMediaFile(ctx, existingFile)
		}
		if isRetryableMediaStorageError(err) {
			*transientStorageFailure = true
		}
		job.FailedCount++
		s.addScanIssue(ctx, job, relative, "error", "file_read_failed", "audio file could not be read completely")
		return
	}
	if stable, err := sourceFileStateUnchanged(path, info); err != nil || !stable {
		if isRetryableMediaStorageError(err) {
			*transientStorageFailure = true
		}
		if existingFile != nil {
			existingFile.Availability = domain.MediaFileAvailabilityChanged
			existingFile.LastSeenAt = &now
			_ = s.repo.UpdateMediaFile(ctx, existingFile)
		}
		job.SkippedCount++
		s.addScanIssue(ctx, job, relative, "warning", "source_busy", "source changed while it was being read; retry after the file is stable")
		return
	}
	modTime := normalizedSourceModTime(info.ModTime())
	if existingFile != nil {
		if strings.EqualFold(existingFile.FileHash, fileHash) {
			existingFile.ObservedFileHash = fileHash
			existingFile.FileSize = info.Size()
			existingFile.FileModTime = &modTime
			existingFile.Availability = domain.MediaFileAvailabilityOnline
			existingFile.LastSeenAt = &now
			existingFile.LastVerifiedAt = &now
			if err := s.repo.UpdateMediaFile(ctx, existingFile); err != nil {
				job.FailedCount++
				s.addScanIssue(ctx, job, relative, "error", "database_write_failed", "source file state could not be refreshed")
				return
			}
			job.ExistingCount++
			return
		}
		existingFile.ObservedFileHash = fileHash
		existingFile.FileSize = info.Size()
		existingFile.FileModTime = &modTime
		existingFile.Availability = domain.MediaFileAvailabilityChanged
		existingFile.LastSeenAt = &now
		existingFile.LastVerifiedAt = &now
		existingFile.ContentRevision++
		if err := s.repo.UpdateMediaFile(ctx, existingFile); err != nil {
			job.FailedCount++
			s.addScanIssue(ctx, job, relative, "error", "database_write_failed", "changed source state could not be recorded")
			return
		}
		job.SkippedCount++
		s.addScanIssue(ctx, job, relative, "warning", "source_changed", "source content changed; the old logical track and analysis identity were preserved for manual review")
		return
	}

	exact, err := s.musicRepo.FindByFileHash(ctx, fileHash)
	if err != nil && !errors.Is(err, repository.ErrMusicNotFound) {
		job.FailedCount++
		s.addScanIssue(ctx, job, relative, "error", "database_read_failed", "duplicate state could not be checked")
		return
	}
	if exact != nil {
		mediaFile := newScannedMediaFile(exact.ID, root, relative, sourceKey, fileHash, info, now)
		if err := s.repo.CreateMediaFile(ctx, mediaFile); err != nil {
			job.FailedCount++
			s.addScanIssue(ctx, job, relative, "error", "database_write_failed", "duplicate source could not be linked to the logical track")
			return
		}
		// A duplicate is not discarded: it becomes a playback fallback and can
		// share future M5 analysis artifacts through the content hash.
		job.DuplicateCount++
		return
	}

	metadata, picture, metadataErr := readScannedAudioMetadata(path, int64(s.config.Scanner.MaxTagSizeMB)*1024*1024, s.maxCoverBytes)
	if metadataErr != nil {
		if isRetryableMediaStorageError(metadataErr) {
			*transientStorageFailure = true
			job.FailedCount++
			s.addScanIssue(ctx, job, relative, "error", "storage_read_interrupted", "network storage interrupted metadata reading; this source will be retried")
			return
		}
		s.addScanIssue(ctx, job, relative, "warning", "metadata_unreadable", fmt.Sprintf("some embedded tags could not be read; safe filename fallbacks were used: %v", metadataErr))
	}
	if strings.TrimSpace(metadata.Title) == "" {
		metadata.Title = strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	}
	if strings.TrimSpace(metadata.Artist) == "" {
		metadata.Artist = unknownArtist
	}
	if stable, err := sourceFileStateUnchanged(path, info); err != nil || !stable {
		if isRetryableMediaStorageError(err) {
			*transientStorageFailure = true
		}
		job.SkippedCount++
		s.addScanIssue(ctx, job, relative, "warning", "source_busy", "source changed while metadata was being read; retry after the file is stable")
		return
	}
	music := &domain.Music{
		Path: path, FileHash: fileHash, Type: domain.MusicTypeSingle, UserID: 0,
		MediaRootID: root.ID, MediaRelativePath: relative, MediaSourceKey: &sourceKey,
		SourceFileSize: info.Size(), SourceFileModTime: &modTime, SourceReadOnly: root.ReadOnly,
	}
	if err := applyCreateMusicMetadata(music, metadata); err != nil {
		job.FailedCount++
		s.addScanIssue(ctx, job, relative, "error", "invalid_metadata", "embedded metadata could not be normalized")
		return
	}
	mediaFile := newScannedMediaFile(0, root, relative, sourceKey, fileHash, info, now)
	if err := s.repo.CreateMusicWithMediaFile(ctx, music, mediaFile); err != nil {
		job.FailedCount++
		s.addScanIssue(ctx, job, relative, "error", "database_write_failed", "track could not be added to the library")
		return
	}
	if picture != nil {
		coverPath, err := saveScannedCover(music.ID, picture, s.managedPath, s.maxCoverBytes)
		if err != nil {
			s.addScanIssue(ctx, job, relative, "warning", "cover_extract_failed", fmt.Sprintf("embedded cover could not be copied to managed storage: %v", err))
		} else if coverPath != "" {
			music.Img = coverPath
			if err := s.musicRepo.Update(ctx, music); err != nil {
				cleanupUploadedFiles([]string{coverPath})
				s.addScanIssue(ctx, job, relative, "warning", "cover_store_failed", "embedded cover was not attached to the track")
			}
		}
	}
	job.ImportedCount++
}

func (s *mediaLibraryService) renewScanLease(ctx context.Context, job *domain.MediaScanJob) error {
	now := time.Now().UTC()
	expiresAt := now.Add(mediaScanLeaseDuration)
	job.HeartbeatAt = &now
	job.LeaseExpiresAt = &expiresAt
	return s.repo.UpdateScanJob(ctx, job)
}

func (s *mediaLibraryService) finishScanJob(ctx context.Context, job *domain.MediaScanJob, status, failureCode string, retryable bool, summary string) {
	now := time.Now().UTC()
	job.Status = status
	job.HeartbeatAt = &now
	job.FinishedAt = &now
	job.NextAttemptAt = nil
	job.FailureCode = failureCode
	job.FailureRetryable = retryable
	job.ErrorSummary = truncateText(summary, 500)
	if err := s.repo.UpdateScanJob(ctx, job); err != nil {
		if errors.Is(err, repository.ErrMediaScanLeaseLost) {
			return
		}
		pklog.Errorf("Failed to finish media scan job %d: %v", job.ID, err)
	}
}

func (s *mediaLibraryService) retryOrFinishScan(ctx context.Context, job *domain.MediaScanJob, code string, retryable bool, summary string) {
	if !retryable || job.Attempt >= s.config.Scanner.RetryMaxAttempts {
		s.finishScanJob(ctx, job, domain.MediaScanStatusFailed, code, retryable, summary)
		return
	}
	delay := time.Duration(s.config.Scanner.RetryInitialSeconds) * time.Second
	for attempt := 1; attempt < job.Attempt; attempt++ {
		delay *= 2
		maximum := time.Duration(s.config.Scanner.RetryMaxSeconds) * time.Second
		if delay >= maximum {
			delay = maximum
			break
		}
	}
	now := time.Now().UTC()
	next := now.Add(delay)
	job.Status = domain.MediaScanStatusRetryWait
	job.HeartbeatAt = &now
	job.FinishedAt = nil
	job.NextAttemptAt = &next
	job.FailureCode = code
	job.FailureRetryable = true
	job.ErrorSummary = truncateText(fmt.Sprintf("%s; retry scheduled for %s", summary, next.Format(time.RFC3339)), 500)
	if err := s.repo.UpdateScanJob(ctx, job); err != nil && !errors.Is(err, repository.ErrMediaScanLeaseLost) {
		pklog.Errorf("Failed to schedule retry for media scan job %d: %v", job.ID, err)
	}
}

func (s *mediaLibraryService) addScanIssue(ctx context.Context, job *domain.MediaScanJob, relative, severity, code, message string) {
	if severity == "warning" {
		job.WarningCount++
	}
	if job.WarningCount+job.FailedCount > mediaScanIssueLimit {
		return
	}
	issue := &domain.MediaScanIssue{
		JobID: job.ID, RelativePath: truncateText(relative, 1000), Severity: severity,
		Code: code, Message: truncateText(message, 500),
	}
	if err := s.repo.CreateScanIssue(ctx, issue); err != nil {
		pklog.Errorf("Failed to record issue for media scan job %d: %v", job.ID, err)
	}
}

func (s *mediaLibraryService) resolveRoot(ctx context.Context, id uint) (*resolvedMediaRoot, error) {
	if id == domain.ManagedMediaRootID {
		if strings.TrimSpace(s.managedPath) == "" {
			return nil, fmt.Errorf("%w: managed directory is not configured", ErrInvalidMediaRoot)
		}
		path, err := filepath.Abs(filepath.Clean(s.managedPath))
		if err != nil {
			return nil, fmt.Errorf("%w: resolve managed directory", ErrInvalidMediaRoot)
		}
		return &resolvedMediaRoot{
			ID: id, Key: domain.ManagedMediaRootKey, Name: "Managed library", Path: path,
			StorageKind: domain.MediaStorageKindManaged, PathSemantics: domain.MediaPathSemanticsAuto,
			Enabled: true, ReadOnly: false,
		}, nil
	}
	root, err := s.repo.FindRootByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return resolvedRootFromDomain(root), nil
}

func (s *mediaLibraryService) ensureRootDoesNotOverlap(ctx context.Context, ignoredID uint, candidate string) error {
	managed, err := s.resolveRoot(ctx, domain.ManagedMediaRootID)
	if err != nil {
		return err
	}
	if filesystemPathsOverlap(managed.Path, candidate) {
		return ErrMediaRootOverlap
	}
	roots, err := s.repo.ListRoots(ctx)
	if err != nil {
		return err
	}
	for _, root := range roots {
		if root.ID != ignoredID && filesystemPathsOverlap(root.Path, candidate) {
			return ErrMediaRootOverlap
		}
	}
	return nil
}

func (s *mediaLibraryService) mediaRootResponse(ctx context.Context, root *resolvedMediaRoot, createdAt, updatedAt time.Time, createdBy uint) (*domain.MediaLibraryRootResponse, error) {
	kind := "read_only"
	if root.ID == domain.ManagedMediaRootID {
		kind = "managed"
	}
	state, err := s.repo.FindRootState(ctx, root.ID)
	if err != nil {
		return nil, err
	}
	return &domain.MediaLibraryRootResponse{
		ID: root.ID, CreatedAt: createdAt, UpdatedAt: updatedAt, Name: root.Name,
		Path: root.Path, Kind: kind, Key: root.Key, StorageKind: root.StorageKind,
		ExpectedFilesystem: root.ExpectedFilesystem, ProbeFile: root.ProbeFile, PathSemantics: root.PathSemantics,
		Enabled: root.Enabled, ReadOnly: root.ReadOnly, CreatedBy: createdBy,
		Health: mediaRootHealthResponse(state),
	}, nil
}

func resolvedRootFromDomain(root *domain.MediaLibraryRoot) *resolvedMediaRoot {
	return &resolvedMediaRoot{
		ID:                 root.ID,
		Key:                root.Key,
		Name:               root.Name,
		Path:               root.Path,
		StorageKind:        normalizeStorageKind(root.StorageKind),
		ExpectedFilesystem: strings.ToLower(strings.TrimSpace(root.ExpectedFilesystem)),
		ProbeFile:          strings.TrimSpace(root.ProbeFile),
		PathSemantics:      normalizeMediaPathSemantics(root.PathSemantics),
		Enabled:            root.Enabled,
		ReadOnly:           true,
	}
}

func validateResolvedRootSpec(root *resolvedMediaRoot) error {
	if err := mediafs.ValidateRootSpec(mediafs.RootSpec{
		Path:               root.Path,
		Kind:               root.StorageKind,
		ExpectedFilesystem: root.ExpectedFilesystem,
		ProbeFile:          root.ProbeFile,
		PathSemantics:      root.PathSemantics,
	}); err != nil {
		return fmt.Errorf("%w: %w", ErrInvalidMediaRoot, err)
	}
	return nil
}

func (s *mediaLibraryService) probeResolvedRoot(ctx context.Context, root *resolvedMediaRoot) (*domain.MediaLibraryRootState, mediafs.Result) {
	result := s.prober.Probe(ctx, mediafs.RootSpec{
		Path:               root.Path,
		Kind:               root.StorageKind,
		ExpectedFilesystem: root.ExpectedFilesystem,
		ProbeFile:          root.ProbeFile,
		PathSemantics:      root.PathSemantics,
	})
	checkedAt := result.CheckedAt
	state := &domain.MediaLibraryRootState{
		RootID:        root.ID,
		Status:        result.Status,
		Code:          truncateText(result.Code, 64),
		Message:       truncateText(result.Message, 500),
		Filesystem:    truncateText(result.Filesystem, 64),
		MountSource:   truncateText(result.MountSource, 500),
		Retryable:     result.Retryable,
		LastCheckedAt: &checkedAt,
	}
	previous, err := s.repo.FindRootState(ctx, root.ID)
	if err == nil && previous != nil {
		state.LastOnlineAt = previous.LastOnlineAt
	}
	if result.Available() {
		state.LastOnlineAt = &checkedAt
	}
	if err := s.repo.UpsertRootState(ctx, state); err != nil {
		pklog.Errorf("Failed to persist health for media root %d: %v", root.ID, err)
	}
	return state, result
}

func mediaRootHealthResponse(state *domain.MediaLibraryRootState) domain.MediaLibraryRootHealthResponse {
	if state == nil {
		return domain.MediaLibraryRootHealthResponse{Status: domain.MediaRootHealthUnknown, Code: "not_checked", Message: "storage has not been checked yet"}
	}
	return domain.MediaLibraryRootHealthResponse{
		Status:        state.Status,
		Code:          state.Code,
		Message:       state.Message,
		Filesystem:    state.Filesystem,
		Retryable:     state.Retryable,
		LastCheckedAt: state.LastCheckedAt,
		LastOnlineAt:  state.LastOnlineAt,
	}
}

func normalizeStorageKind(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return domain.MediaStorageKindAuto
	}
	return value
}

func normalizeMediaPathSemantics(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return domain.MediaPathSemanticsAuto
	}
	return value
}

func newMediaRootKey() string {
	buffer := make([]byte, 16)
	if _, err := rand.Read(buffer); err == nil {
		return "root-" + hex.EncodeToString(buffer)
	}
	// The database ID remains unique; this fallback only covers an extremely
	// rare entropy-source failure and is replaced by the service on collision.
	return fmt.Sprintf("root-%d", time.Now().UTC().UnixNano())
}

func newMediaWorkerID() string {
	buffer := make([]byte, 16)
	if _, err := rand.Read(buffer); err == nil {
		return hex.EncodeToString(buffer)
	}
	return fmt.Sprintf("worker-%d", time.Now().UTC().UnixNano())
}

func newScannedMediaFile(musicID uint, root *resolvedMediaRoot, relative, sourceKey, fileHash string, info fs.FileInfo, now time.Time) *domain.MediaFile {
	modTime := normalizedSourceModTime(info.ModTime())
	return &domain.MediaFile{
		MusicID:          musicID,
		RootID:           root.ID,
		RelativePath:     relative,
		SourceKey:        sourceKey,
		FileHash:         fileHash,
		ObservedFileHash: fileHash,
		FileSize:         info.Size(),
		FileModTime:      &modTime,
		ReadOnly:         root.ReadOnly,
		Availability:     domain.MediaFileAvailabilityOnline,
		ContentRevision:  1,
		LastSeenAt:       &now,
		LastVerifiedAt:   &now,
	}
}

func sourceNeedsHashRecheck(lastVerifiedAt *time.Time, hours int, now time.Time) bool {
	if hours == 0 {
		return false
	}
	return lastVerifiedAt == nil || now.Sub(*lastVerifiedAt) >= time.Duration(hours)*time.Hour
}

func normalizeMediaRootPath(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" || !filepath.IsAbs(value) {
		return "", fmt.Errorf("%w: path must be absolute inside the server or container", ErrInvalidMediaRoot)
	}
	path := filepath.Clean(value)
	if sameFilesystemPath(path, filepath.Dir(path)) && (runtime.GOOS != "windows" || !strings.HasPrefix(path, `\\`)) {
		return "", fmt.Errorf("%w: filesystem root cannot be registered", ErrInvalidMediaRoot)
	}
	return path, nil
}

func filesystemPathsOverlap(left, right string) bool {
	leftAbs, leftErr := filepath.Abs(filepath.Clean(left))
	rightAbs, rightErr := filepath.Abs(filepath.Clean(right))
	if leftErr != nil || rightErr != nil {
		return false
	}
	return pathContains(leftAbs, rightAbs) || pathContains(rightAbs, leftAbs)
}

func pathContains(root, candidate string) bool {
	relative, err := filepath.Rel(root, candidate)
	if err != nil {
		return false
	}
	if relative == "." {
		return true
	}
	return relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) && !filepath.IsAbs(relative)
}

func sameFilesystemPath(left, right string) bool {
	left = filepath.Clean(left)
	right = filepath.Clean(right)
	if absolute, err := filepath.Abs(left); err == nil {
		left = absolute
	}
	if absolute, err := filepath.Abs(right); err == nil {
		right = absolute
	}
	if runtime.GOOS == "windows" {
		return strings.EqualFold(left, right)
	}
	return left == right
}

func securePathWithinRoot(rootPath, relativePath string) (string, error) {
	if relativePath == "" || filepath.IsAbs(relativePath) {
		return "", ErrMediaNotFound
	}
	relative := filepath.Clean(filepath.FromSlash(relativePath))
	if relative == "." || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", ErrMediaNotFound
	}
	rootAbs, err := filepath.Abs(filepath.Clean(rootPath))
	if err != nil {
		return "", err
	}
	candidateAbs, err := filepath.Abs(filepath.Join(rootAbs, relative))
	if err != nil || !pathContains(rootAbs, candidateAbs) {
		return "", ErrMediaNotFound
	}
	resolvedRoot, err := filepath.EvalSymlinks(rootAbs)
	if err != nil {
		return "", err
	}
	resolvedCandidate, err := filepath.EvalSymlinks(candidateAbs)
	if err != nil || !pathContains(resolvedRoot, resolvedCandidate) {
		return "", ErrMediaNotFound
	}
	info, err := os.Lstat(resolvedCandidate)
	if err != nil || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
		return "", ErrMediaNotFound
	}
	return resolvedCandidate, nil
}

func mediaSourceKey(rootID uint, relative, pathSemantics string) string {
	value := strconv.FormatUint(uint64(rootID), 10) + "\x00" + filepath.ToSlash(relative)
	caseInsensitive := pathSemantics == domain.MediaPathSemanticsCaseInsensitive ||
		(pathSemantics == domain.MediaPathSemanticsAuto && runtime.GOOS == "windows")
	if caseInsensitive {
		value = strings.ToLower(value)
	}
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

func safeRelativePath(root, path string) string {
	relative, err := filepath.Rel(root, path)
	if err != nil || relative == "." || filepath.IsAbs(relative) || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return filepath.Base(path)
	}
	return filepath.ToSlash(relative)
}

func sameFileTimestamp(stored *time.Time, current time.Time) bool {
	return stored != nil && normalizedSourceModTime(*stored).Equal(normalizedSourceModTime(current))
}

// PostgreSQL stores timestamps with microsecond precision while several local
// filesystems expose nanoseconds. Normalizing both sides avoids rehashing every
// unchanged file merely because the database rounded its modification time.
func normalizedSourceModTime(value time.Time) time.Time {
	return value.UTC().Truncate(time.Microsecond)
}

func sourceFileStateUnchanged(path string, before fs.FileInfo) (bool, error) {
	after, err := os.Stat(path)
	if err != nil {
		return false, err
	}
	// SameFile catches an atomic path replacement even when a remote writer
	// deliberately preserves size and mtime between hashing and tag parsing.
	return after.Mode().IsRegular() && os.SameFile(before, after) &&
		before.Size() == after.Size() && before.ModTime().Equal(after.ModTime()), nil
}

func isScannableAudioName(name string) bool {
	_, ok := allowedAudioExts[strings.ToLower(filepath.Ext(name))]
	return ok
}

func validateScannedAudio(path string) error {
	file, err := os.Open(filepath.Clean(path))
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()
	buf := make([]byte, signatureReadSize)
	n, err := io.ReadFull(file, buf)
	if err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, io.ErrUnexpectedEOF) {
		return err
	}
	if !isSupportedAudioSignature(buf[:n]) {
		return ErrInvalidMediaFile
	}
	return nil
}

func hashScannedFile(ctx context.Context, path string, heartbeat func() error) (string, error) {
	file, err := os.Open(filepath.Clean(path))
	if err != nil {
		return "", err
	}
	defer func() { _ = file.Close() }()
	hasher := sha256.New()
	buf := make([]byte, 1024*1024)
	lastHeartbeat := time.Now()
	for {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		n, readErr := file.Read(buf)
		if n > 0 {
			if _, err := hasher.Write(buf[:n]); err != nil {
				return "", err
			}
		}
		if heartbeat != nil && time.Since(lastHeartbeat) >= 10*time.Second {
			if err := heartbeat(); err != nil {
				return "", err
			}
			lastHeartbeat = time.Now()
		}
		if errors.Is(readErr, io.EOF) {
			break
		}
		if readErr != nil {
			return "", readErr
		}
	}
	return hex.EncodeToString(hasher.Sum(nil)), nil
}

func truncateText(value string, max int) string {
	value = strings.TrimSpace(value)
	if len(value) <= max {
		return value
	}
	value = value[:max]
	for len(value) > 0 && !utf8.ValidString(value) {
		value = value[:len(value)-1]
	}
	return value
}
