package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
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
	return errors.Is(err, ErrMediaStorageUnavailable) || mediafs.IsTransientError(err)
}

func isRetryableMediaStorageError(err error) bool {
	return mediafs.IsRetryableError(err)
}

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
	analyzer      MusicAnalysisScheduler

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

// newMediaLibraryService constructs the path, root-health and scanner core.
// It stays private because the public music subsystem constructor completes
// the analysis scheduling cycle before any service can be observed by callers.
func newMediaLibraryService(repo repository.MediaLibraryRepository, musicRepo repository.MusicRepository, cfg config.LibraryConfig, serverConfig config.ServerConfig, prober mediafs.Prober) *mediaLibraryService {
	return &mediaLibraryService{
		repo:          repo,
		musicRepo:     musicRepo,
		config:        cfg,
		prober:        prober,
		managedPath:   strings.TrimSpace(serverConfig.UploadDir),
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
