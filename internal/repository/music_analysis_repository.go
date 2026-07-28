package repository

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/kyangconn/music-online-go/internal/domain"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	ErrAnalysisJobNotFound  = errors.New("music analysis job not found")
	ErrAnalysisJobLeaseLost = errors.New("music analysis job lease was lost")
	ErrAnalysisQueueFull    = errors.New("music analysis queue is full")
	ErrAnalysisJobActive    = errors.New("music analysis job is already active")
	ErrAnalysisJobCancelled = errors.New("music analysis job cancellation was requested")
)

var activeAnalysisStatuses = [...]string{
	domain.AnalysisStatusPending,
	domain.AnalysisStatusRunning,
}

type MusicAnalysisRepository interface {
	FindCandidate(ctx context.Context, musicID uint) (*domain.AnalysisMusicCandidate, error)
	ListCandidates(ctx context.Context, afterID uint, limit int) ([]*domain.AnalysisMusicCandidate, error)
	Enqueue(ctx context.Context, job *domain.MusicAnalysisJob, force bool, queueLimit int) (*domain.MusicAnalysisJob, bool, error)
	ClaimNext(ctx context.Context, owner string, leaseDuration time.Duration) (*domain.MusicAnalysisJob, bool, error)
	FindJob(ctx context.Context, id uint) (*domain.MusicAnalysisJob, error)
	ListJobs(ctx context.Context, params domain.AnalysisJobListParams) ([]*domain.MusicAnalysisJob, int64, error)
	LatestAudioJobsByMusicIDs(ctx context.Context, musicIDs []uint) (map[uint]*domain.MusicAnalysisJob, error)
	Heartbeat(ctx context.Context, job *domain.MusicAnalysisJob, leaseDuration time.Duration) (bool, error)
	Complete(ctx context.Context, job *domain.MusicAnalysisJob) error
	RequestCancellation(ctx context.Context, id uint) (*domain.MusicAnalysisJob, error)
	RecoverExpired(ctx context.Context) error
	MarkSupersededAudio(ctx context.Context, musicID uint, currentHash string) error
	FindCachedAnalysis(ctx context.Context, fileHash, analyzerID, analyzerVersion, modelVersion string) (*domain.MusicAudioAnalysis, error)
	StoreAnalysis(ctx context.Context, analysis *domain.MusicAudioAnalysis) (*domain.MusicAudioAnalysis, error)
	Metrics(ctx context.Context) (*domain.AnalysisQueueMetrics, error)
}

type musicAnalysisRepository struct {
	db *gorm.DB
}

func NewMusicAnalysisRepository(db *gorm.DB) MusicAnalysisRepository {
	return &musicAnalysisRepository{db: db}
}

func (r *musicAnalysisRepository) FindCandidate(ctx context.Context, musicID uint) (*domain.AnalysisMusicCandidate, error) {
	var music domain.Music
	if err := r.db.WithContext(ctx).First(&music, musicID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrMusicNotFound
		}
		return nil, fmt.Errorf("find analysis music: %w", err)
	}
	candidates, err := r.candidatesForMusics(ctx, []*domain.Music{&music})
	if err != nil {
		return nil, err
	}
	return candidates[0], nil
}

func (r *musicAnalysisRepository) ListCandidates(ctx context.Context, afterID uint, limit int) ([]*domain.AnalysisMusicCandidate, error) {
	if limit <= 0 || limit > 500 {
		limit = 200
	}
	var musics []*domain.Music
	if err := r.db.WithContext(ctx).Where("id > ?", afterID).Order("id ASC").Limit(limit).Find(&musics).Error; err != nil {
		return nil, fmt.Errorf("list analysis music candidates: %w", err)
	}
	return r.candidatesForMusics(ctx, musics)
}

func (r *musicAnalysisRepository) candidatesForMusics(ctx context.Context, musics []*domain.Music) ([]*domain.AnalysisMusicCandidate, error) {
	result := make([]*domain.AnalysisMusicCandidate, 0, len(musics))
	if len(musics) == 0 {
		return result, nil
	}
	ids := make([]uint, 0, len(musics))
	for _, music := range musics {
		ids = append(ids, music.ID)
	}
	var mediaFiles []*domain.MediaFile
	if err := r.db.WithContext(ctx).Where("music_id IN ?", ids).
		Order("music_id ASC, CASE availability WHEN 'online' THEN 0 WHEN 'unknown' THEN 1 ELSE 2 END, root_id ASC, id ASC").
		Find(&mediaFiles).Error; err != nil {
		return nil, fmt.Errorf("list analysis media sources: %w", err)
	}
	byMusic := make(map[uint][]*domain.MediaFile, len(musics))
	for _, mediaFile := range mediaFiles {
		byMusic[mediaFile.MusicID] = append(byMusic[mediaFile.MusicID], mediaFile)
	}
	for _, music := range musics {
		candidate := &domain.AnalysisMusicCandidate{Music: music, FileHash: normalizedHash(music.FileHash)}
		for _, mediaFile := range byMusic[music.ID] {
			if mediaFile.Availability == domain.MediaFileAvailabilityChanged || mediaFile.Availability == domain.MediaFileAvailabilityMissing {
				continue
			}
			fileHash := normalizedHash(mediaFile.FileHash)
			if fileHash == "" || (candidate.FileHash != "" && fileHash != candidate.FileHash) {
				continue
			}
			id := mediaFile.ID
			candidate.MediaFileID = &id
			candidate.FileHash = fileHash
			candidate.ContentRevision = mediaFile.ContentRevision
			break
		}
		result = append(result, candidate)
	}
	return result, nil
}

func (r *musicAnalysisRepository) Enqueue(ctx context.Context, job *domain.MusicAnalysisJob, force bool, queueLimit int) (*domain.MusicAnalysisJob, bool, error) {
	if job == nil || job.MusicID == 0 || job.IdempotencyKey == "" || !domain.IsAnalysisJobKind(job.Kind) {
		return nil, false, errors.New("invalid music analysis job")
	}
	if job.Status == "" {
		job.Status = domain.AnalysisStatusPending
	}
	if job.MaxAttempts <= 0 {
		job.MaxAttempts = 1
	}
	var queued *domain.MusicAnalysisJob
	var changed bool
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var existing domain.MusicAnalysisJob
		lookup := tx.Where("idempotency_key = ?", job.IdempotencyKey).Limit(1).Find(&existing)
		if lookup.Error != nil {
			return lookup.Error
		}
		if lookup.RowsAffected > 0 {
			if !force {
				queued = &existing
				return nil
			}
			if existing.Status == domain.AnalysisStatusPending || existing.Status == domain.AnalysisStatusRunning {
				return ErrAnalysisJobActive
			}
			now := time.Now().UTC()
			result := tx.Model(&domain.MusicAnalysisJob{}).
				Where("id = ? AND status NOT IN ?", existing.ID, activeAnalysisStatuses[:]).
				Updates(map[string]any{
					"status": domain.AnalysisStatusPending, "attempt": 0, "available_at": now,
					"cancel_requested": false, "started_at": nil, "heartbeat_at": nil, "finished_at": nil,
					"lease_owner": "", "lease_expires_at": nil, "error_code": "", "error_summary": "",
					"processing_ms": 0, "analysis_id": nil, "observed_file_hash": "",
				})
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected == 0 {
				return ErrAnalysisJobActive
			}
			if err := tx.First(&existing, existing.ID).Error; err != nil {
				return err
			}
			queued = &existing
			changed = true
			return nil
		}
		if queueLimit > 0 {
			var active int64
			if err := tx.Model(&domain.MusicAnalysisJob{}).Where("status IN ?", activeAnalysisStatuses[:]).Count(&active).Error; err != nil {
				return err
			}
			if active >= int64(queueLimit) {
				return ErrAnalysisQueueFull
			}
		}
		if job.AvailableAt == nil {
			now := time.Now().UTC()
			job.AvailableAt = &now
		}
		if err := tx.Create(job).Error; err != nil {
			return err
		}
		queued = job
		changed = true
		return nil
	})
	if err == nil || errors.Is(err, ErrAnalysisQueueFull) || errors.Is(err, ErrAnalysisJobActive) {
		return queued, changed, err
	}
	// A concurrent process may have inserted the same idempotency key. Resolve
	// that race to the stable existing job without parsing driver errors.
	var existing domain.MusicAnalysisJob
	lookup := r.db.WithContext(ctx).Where("idempotency_key = ?", job.IdempotencyKey).Limit(1).Find(&existing)
	if lookup.Error == nil && lookup.RowsAffected > 0 {
		return &existing, false, nil
	}
	return nil, false, fmt.Errorf("enqueue music analysis: %w", err)
}

func (r *musicAnalysisRepository) ClaimNext(ctx context.Context, owner string, leaseDuration time.Duration) (*domain.MusicAnalysisJob, bool, error) {
	var claimed *domain.MusicAnalysisJob
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		now := time.Now().UTC()
		query := tx.Where(
			"status = ? AND cancel_requested = ? AND (available_at IS NULL OR available_at <= ?)",
			domain.AnalysisStatusPending, false, now,
		).Order("id ASC")
		if tx.Dialector.Name() == "postgres" {
			query = query.Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"})
		}
		var job domain.MusicAnalysisJob
		lookup := query.Limit(1).Find(&job)
		if lookup.Error != nil {
			return lookup.Error
		}
		if lookup.RowsAffected == 0 {
			return nil
		}
		expiresAt := now.Add(leaseDuration)
		result := tx.Model(&domain.MusicAnalysisJob{}).
			Where("id = ? AND status = ? AND cancel_requested = ?", job.ID, domain.AnalysisStatusPending, false).
			Updates(map[string]any{
				"status": domain.AnalysisStatusRunning, "attempt": gorm.Expr("attempt + 1"),
				"available_at": nil, "started_at": gorm.Expr("COALESCE(started_at, ?)", now),
				"heartbeat_at": now, "finished_at": nil, "lease_owner": owner,
				"lease_expires_at": expiresAt, "lease_generation": gorm.Expr("lease_generation + 1"),
				"error_code": "", "error_summary": "", "processing_ms": 0,
			})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return nil
		}
		if err := tx.First(&job, job.ID).Error; err != nil {
			return err
		}
		claimed = &job
		return nil
	})
	if err != nil {
		return nil, false, fmt.Errorf("claim music analysis: %w", err)
	}
	return claimed, claimed != nil, nil
}

func (r *musicAnalysisRepository) FindJob(ctx context.Context, id uint) (*domain.MusicAnalysisJob, error) {
	var job domain.MusicAnalysisJob
	if err := r.db.WithContext(ctx).Preload("Analysis").First(&job, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrAnalysisJobNotFound
		}
		return nil, fmt.Errorf("find music analysis job: %w", err)
	}
	return &job, nil
}

func (r *musicAnalysisRepository) ListJobs(ctx context.Context, params domain.AnalysisJobListParams) ([]*domain.MusicAnalysisJob, int64, error) {
	query := r.db.WithContext(ctx).Model(&domain.MusicAnalysisJob{})
	if params.MusicID != nil {
		query = query.Where("music_id = ?", *params.MusicID)
	}
	if params.Kind != "" {
		query = query.Where("kind = ?", params.Kind)
	}
	if params.Status != "" {
		query = query.Where("status = ?", params.Status)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count music analysis jobs: %w", err)
	}
	page, pageSize := params.Page, params.PageSize
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 20
	}
	var jobs []*domain.MusicAnalysisJob
	if err := query.Preload("Analysis").Order("id DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&jobs).Error; err != nil {
		return nil, 0, fmt.Errorf("list music analysis jobs: %w", err)
	}
	return jobs, total, nil
}

func (r *musicAnalysisRepository) LatestAudioJobsByMusicIDs(ctx context.Context, musicIDs []uint) (map[uint]*domain.MusicAnalysisJob, error) {
	result := make(map[uint]*domain.MusicAnalysisJob)
	if len(musicIDs) == 0 {
		return result, nil
	}
	latestIDs := r.db.WithContext(ctx).Model(&domain.MusicAnalysisJob{}).
		Select("MAX(id)").
		Where("music_id IN ? AND kind = ?", musicIDs, domain.AnalysisJobKindAudio).
		Group("music_id")
	var jobs []*domain.MusicAnalysisJob
	if err := r.db.WithContext(ctx).Where("id IN (?)", latestIDs).Find(&jobs).Error; err != nil {
		return nil, fmt.Errorf("list latest music analyses: %w", err)
	}
	for _, job := range jobs {
		result[job.MusicID] = job
	}
	return result, nil
}

func (r *musicAnalysisRepository) Heartbeat(ctx context.Context, job *domain.MusicAnalysisJob, leaseDuration time.Duration) (bool, error) {
	now := time.Now().UTC()
	expiresAt := now.Add(leaseDuration)
	result := r.db.WithContext(ctx).Model(&domain.MusicAnalysisJob{}).
		Where("id = ? AND status = ? AND lease_owner = ? AND lease_generation = ?", job.ID, domain.AnalysisStatusRunning, job.LeaseOwner, job.LeaseGeneration).
		Updates(map[string]any{"heartbeat_at": now, "lease_expires_at": expiresAt})
	if result.Error != nil {
		return false, result.Error
	}
	if result.RowsAffected == 0 {
		return false, ErrAnalysisJobLeaseLost
	}
	job.HeartbeatAt = &now
	job.LeaseExpiresAt = &expiresAt
	var state struct {
		CancelRequested bool
		Status          string
	}
	if err := r.db.WithContext(ctx).Model(&domain.MusicAnalysisJob{}).Select("cancel_requested", "status").First(&state, job.ID).Error; err != nil {
		return false, err
	}
	return state.CancelRequested || state.Status == domain.AnalysisStatusCancelled, nil
}

func (r *musicAnalysisRepository) Complete(ctx context.Context, job *domain.MusicAnalysisJob) error {
	if job.Status == domain.AnalysisStatusRunning || !domain.IsAnalysisJobStatus(job.Status) ||
		(job.Status == domain.AnalysisStatusPending && job.AvailableAt == nil) {
		return errors.New("analysis completion requires a terminal status or scheduled retry")
	}
	query := r.db.WithContext(ctx).Model(&domain.MusicAnalysisJob{}).
		Where("id = ? AND status = ? AND lease_owner = ? AND lease_generation = ?", job.ID, domain.AnalysisStatusRunning, job.LeaseOwner, job.LeaseGeneration)
	// Cancellation wins over every normal completion, including a retry that
	// races with an administrator request. A cancellation finalization itself
	// is allowed to consume the flagged lease.
	if job.Status != domain.AnalysisStatusCancelled && job.ErrorCode != "cancelled" {
		query = query.Where("cancel_requested = ?", false)
	}
	result := query.
		Updates(map[string]any{
			"status": job.Status, "analysis_id": job.AnalysisID, "observed_file_hash": job.ObservedFileHash,
			"heartbeat_at": job.HeartbeatAt, "finished_at": job.FinishedAt, "available_at": job.AvailableAt,
			"error_code": job.ErrorCode, "error_summary": job.ErrorSummary, "processing_ms": job.ProcessingMS,
			"lease_owner": "", "lease_expires_at": nil,
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		var state struct {
			CancelRequested bool
			Status          string
			LeaseOwner      string
			LeaseGeneration uint64
		}
		if err := r.db.WithContext(ctx).Model(&domain.MusicAnalysisJob{}).
			Select("cancel_requested", "status", "lease_owner", "lease_generation").First(&state, job.ID).Error; err == nil &&
			state.CancelRequested && state.Status == domain.AnalysisStatusRunning &&
			state.LeaseOwner == job.LeaseOwner && state.LeaseGeneration == job.LeaseGeneration {
			return ErrAnalysisJobCancelled
		}
		return ErrAnalysisJobLeaseLost
	}
	job.LeaseOwner = ""
	job.LeaseExpiresAt = nil
	return nil
}

func (r *musicAnalysisRepository) RequestCancellation(ctx context.Context, id uint) (*domain.MusicAnalysisJob, error) {
	job, err := r.FindJob(ctx, id)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	switch job.Status {
	case domain.AnalysisStatusPending:
		result := r.db.WithContext(ctx).Model(&domain.MusicAnalysisJob{}).
			Where("id = ? AND status = ?", id, domain.AnalysisStatusPending).
			Updates(map[string]any{"status": domain.AnalysisStatusCancelled, "cancel_requested": true, "finished_at": now})
		if result.Error != nil {
			return nil, result.Error
		}
		if result.RowsAffected == 0 {
			if err := r.db.WithContext(ctx).Model(&domain.MusicAnalysisJob{}).Where("id = ? AND status = ?", id, domain.AnalysisStatusRunning).
				Update("cancel_requested", true).Error; err != nil {
				return nil, err
			}
		}
	case domain.AnalysisStatusRunning:
		if err := r.db.WithContext(ctx).Model(&domain.MusicAnalysisJob{}).Where("id = ? AND status = ?", id, domain.AnalysisStatusRunning).
			Update("cancel_requested", true).Error; err != nil {
			return nil, err
		}
	}
	return r.FindJob(ctx, id)
}

func (r *musicAnalysisRepository) RecoverExpired(ctx context.Context) error {
	now := time.Now().UTC()
	if err := r.db.WithContext(ctx).Model(&domain.MusicAnalysisJob{}).
		Where("status = ? AND cancel_requested = ? AND (lease_expires_at IS NULL OR lease_expires_at <= ?)", domain.AnalysisStatusRunning, true, now).
		Updates(map[string]any{"status": domain.AnalysisStatusCancelled, "finished_at": now, "lease_owner": "", "lease_expires_at": nil}).Error; err != nil {
		return fmt.Errorf("recover cancelled music analysis jobs: %w", err)
	}
	if err := r.db.WithContext(ctx).Model(&domain.MusicAnalysisJob{}).
		Where("status = ? AND cancel_requested = ? AND attempt >= max_attempts AND (lease_expires_at IS NULL OR lease_expires_at <= ?)", domain.AnalysisStatusRunning, false, now).
		Updates(map[string]any{
			"status": domain.AnalysisStatusFailed, "finished_at": now, "heartbeat_at": now,
			"error_code": "worker_lease_expired", "error_summary": "analysis worker lease expired after the final attempt",
			"lease_owner": "", "lease_expires_at": nil,
		}).Error; err != nil {
		return fmt.Errorf("fail expired music analysis jobs: %w", err)
	}
	if err := r.db.WithContext(ctx).Model(&domain.MusicAnalysisJob{}).
		Where("status = ? AND cancel_requested = ? AND attempt < max_attempts AND (lease_expires_at IS NULL OR lease_expires_at <= ?)", domain.AnalysisStatusRunning, false, now).
		Updates(map[string]any{
			"status": domain.AnalysisStatusPending, "available_at": now, "heartbeat_at": now,
			"error_code": "worker_lease_expired", "error_summary": "analysis worker lease expired; the job will be retried",
			"lease_owner": "", "lease_expires_at": nil,
		}).Error; err != nil {
		return fmt.Errorf("recover expired music analysis jobs: %w", err)
	}
	return nil
}

func (r *musicAnalysisRepository) MarkSupersededAudio(ctx context.Context, musicID uint, currentHash string) error {
	currentHash = normalizedHash(currentHash)
	baseQuery := func() *gorm.DB {
		query := r.db.WithContext(ctx).Model(&domain.MusicAnalysisJob{}).
			Where("music_id = ? AND kind = ?", musicID, domain.AnalysisJobKindAudio)
		if currentHash != "" {
			query = query.Where("LOWER(file_hash) <> ?", currentHash)
		}
		return query
	}
	if err := baseQuery().Where("status = ?", domain.AnalysisStatusRunning).
		Updates(map[string]any{"cancel_requested": true, "error_code": "content_superseded"}).Error; err != nil {
		return err
	}
	now := time.Now().UTC()
	return baseQuery().Where("status IN ?", []string{
		domain.AnalysisStatusPending, domain.AnalysisStatusSucceeded, domain.AnalysisStatusFailed,
	}).Updates(map[string]any{
		"status": domain.AnalysisStatusStale, "finished_at": now,
		"error_code": "content_superseded", "error_summary": "track content changed after this analysis job was created",
	}).Error
}

func (r *musicAnalysisRepository) FindCachedAnalysis(ctx context.Context, fileHash, analyzerID, analyzerVersion, modelVersion string) (*domain.MusicAudioAnalysis, error) {
	var analysis domain.MusicAudioAnalysis
	query := r.db.WithContext(ctx).Where(
		"file_hash = ? AND analyzer_id = ? AND analyzer_version = ? AND model_version = ? AND status = ?",
		normalizedHash(fileHash), analyzerID, analyzerVersion, modelVersion, domain.AnalysisStatusSucceeded,
	).Limit(1).Find(&analysis)
	if query.Error != nil {
		return nil, fmt.Errorf("find cached music analysis: %w", query.Error)
	}
	if query.RowsAffected == 0 {
		return nil, nil
	}
	return &analysis, nil
}

func (r *musicAnalysisRepository) StoreAnalysis(ctx context.Context, analysis *domain.MusicAudioAnalysis) (*domain.MusicAudioAnalysis, error) {
	if analysis == nil {
		return nil, errors.New("music audio analysis is nil")
	}
	analysis.FileHash = normalizedHash(analysis.FileHash)
	err := r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "file_hash"}, {Name: "analyzer_id"}, {Name: "analyzer_version"}, {Name: "model_version"}},
		DoNothing: true,
	}).Create(analysis).Error
	if err != nil {
		return nil, fmt.Errorf("store music analysis: %w", err)
	}
	var stored domain.MusicAudioAnalysis
	if err := r.db.WithContext(ctx).Where(
		"file_hash = ? AND analyzer_id = ? AND analyzer_version = ? AND model_version = ?",
		analysis.FileHash, analysis.AnalyzerID, analysis.AnalyzerVersion, analysis.ModelVersion,
	).First(&stored).Error; err != nil {
		return nil, fmt.Errorf("reload music analysis: %w", err)
	}
	return &stored, nil
}

func (r *musicAnalysisRepository) Metrics(ctx context.Context) (*domain.AnalysisQueueMetrics, error) {
	metrics := &domain.AnalysisQueueMetrics{Statuses: map[string]int64{
		domain.AnalysisStatusPending: 0, domain.AnalysisStatusRunning: 0,
		domain.AnalysisStatusSucceeded: 0, domain.AnalysisStatusFailed: 0,
		domain.AnalysisStatusStale: 0, domain.AnalysisStatusCancelled: 0,
	}}
	type statusCount struct {
		Status string
		Count  int64
	}
	var counts []statusCount
	if err := r.db.WithContext(ctx).Model(&domain.MusicAnalysisJob{}).Select("status, COUNT(*) AS count").Group("status").Scan(&counts).Error; err != nil {
		return nil, fmt.Errorf("count analysis queue statuses: %w", err)
	}
	for _, count := range counts {
		metrics.Statuses[count.Status] = count.Count
	}
	metrics.QueueLength = metrics.Statuses[domain.AnalysisStatusPending] + metrics.Statuses[domain.AnalysisStatusRunning]
	var averageProcessing float64
	if err := r.db.WithContext(ctx).Model(&domain.MusicAnalysisJob{}).Where("status = ?", domain.AnalysisStatusSucceeded).
		Select("COALESCE(AVG(processing_ms), 0)").Scan(&averageProcessing).Error; err != nil {
		return nil, fmt.Errorf("average analysis processing time: %w", err)
	}
	metrics.AverageProcessing = int64(math.Round(averageProcessing))
	completed := metrics.Statuses[domain.AnalysisStatusSucceeded] + metrics.Statuses[domain.AnalysisStatusFailed]
	if completed > 0 {
		metrics.FailureRate = float64(metrics.Statuses[domain.AnalysisStatusFailed]) / float64(completed)
	}
	return metrics, nil
}

func normalizedHash(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

// deleteMusicAnalysisState removes per-track jobs first, then deletes only
// artifacts no longer referenced by another track. Identical content may share
// one cached artifact, so deleting one music row must not invalidate the other.
func deleteMusicAnalysisState(tx *gorm.DB, musicIDs []uint) error {
	if len(musicIDs) == 0 || !tx.Migrator().HasTable(&domain.MusicAnalysisJob{}) {
		return nil
	}
	var analysisIDs []uint
	if err := tx.Model(&domain.MusicAnalysisJob{}).Where("music_id IN ? AND analysis_id IS NOT NULL", musicIDs).
		Distinct("analysis_id").Pluck("analysis_id", &analysisIDs).Error; err != nil {
		return fmt.Errorf("list music analysis artifacts for deletion: %w", err)
	}
	if err := tx.Where("music_id IN ?", musicIDs).Delete(&domain.MusicAnalysisJob{}).Error; err != nil {
		return fmt.Errorf("delete music analysis jobs: %w", err)
	}
	if len(analysisIDs) == 0 {
		return nil
	}
	if err := tx.Where("id IN ? AND NOT EXISTS (?)", analysisIDs,
		tx.Model(&domain.MusicAnalysisJob{}).Select("1").Where("music_analysis_jobs.analysis_id = music_audio_analyses.id"),
	).Delete(&domain.MusicAudioAnalysis{}).Error; err != nil {
		return fmt.Errorf("delete orphaned music analysis artifacts: %w", err)
	}
	return nil
}
