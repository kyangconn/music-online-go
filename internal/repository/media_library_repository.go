package repository

import (
	"context"
	"errors"
	"time"

	"github.com/kyangconn/music-online-go/internal/domain"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	ErrMediaRootNotFound   = errors.New("media library root not found")
	ErrMediaRootInUse      = errors.New("media library root is in use")
	ErrMediaScanNotFound   = errors.New("media scan job not found")
	ErrMediaScanInProgress = errors.New("a media scan is already active for this root")
	ErrMediaScanLeaseLost  = errors.New("media scan worker lease was lost")
	ErrMediaFileNotFound   = errors.New("media file not found")
)

var activeMediaScanStatuses = []string{
	domain.MediaScanStatusPending,
	domain.MediaScanStatusRunning,
	domain.MediaScanStatusRetryWait,
}

type MediaLibraryRepository interface {
	ListRoots(ctx context.Context) ([]*domain.MediaLibraryRoot, error)
	FindRootByID(ctx context.Context, id uint) (*domain.MediaLibraryRoot, error)
	CreateRoot(ctx context.Context, root *domain.MediaLibraryRoot) error
	UpdateRoot(ctx context.Context, root *domain.MediaLibraryRoot) error
	DeleteRoot(ctx context.Context, id uint) error
	CountMusicByRoot(ctx context.Context, rootID uint) (int64, error)
	HasActiveScanForRoot(ctx context.Context, rootID uint) (bool, error)
	FindRootState(ctx context.Context, rootID uint) (*domain.MediaLibraryRootState, error)
	UpsertRootState(ctx context.Context, state *domain.MediaLibraryRootState) error

	FindMediaFileBySourceKey(ctx context.Context, sourceKey string) (*domain.MediaFile, error)
	ListMediaFilesByMusicID(ctx context.Context, musicID uint) ([]*domain.MediaFile, error)
	HasReadOnlyMediaFile(ctx context.Context, musicID uint) (bool, error)
	CreateMediaFile(ctx context.Context, mediaFile *domain.MediaFile) error
	UpdateMediaFile(ctx context.Context, mediaFile *domain.MediaFile) error
	MarkRootFilesMissingBefore(ctx context.Context, rootID uint, observedAfter time.Time) error
	CreateMusicWithMediaFile(ctx context.Context, music *domain.Music, mediaFile *domain.MediaFile) error
	ReplaceManagedMediaFile(ctx context.Context, music *domain.Music, mediaFile *domain.MediaFile) error

	CreateScanJob(ctx context.Context, job *domain.MediaScanJob) error
	ClaimNextScanJob(ctx context.Context, owner string, leaseDuration time.Duration) (*domain.MediaScanJob, bool, error)
	FindScanJob(ctx context.Context, id uint) (*domain.MediaScanJob, error)
	ListScanJobs(ctx context.Context, rootID *uint, page, pageSize int) ([]*domain.MediaScanJob, int64, error)
	UpdateScanJob(ctx context.Context, job *domain.MediaScanJob) error
	RequestScanCancellation(ctx context.Context, id uint) (*domain.MediaScanJob, error)
	IsScanCancellationRequested(ctx context.Context, id uint) (bool, error)
	RecoverExpiredScanJobs(ctx context.Context) error
	CreateScanIssue(ctx context.Context, issue *domain.MediaScanIssue) error
	ListScanIssues(ctx context.Context, jobID uint, limit int) ([]*domain.MediaScanIssue, error)
}

type mediaLibraryRepository struct {
	db *gorm.DB
}

func NewMediaLibraryRepository(db *gorm.DB) MediaLibraryRepository {
	return &mediaLibraryRepository{db: db}
}

func (r *mediaLibraryRepository) ListRoots(ctx context.Context) ([]*domain.MediaLibraryRoot, error) {
	var roots []*domain.MediaLibraryRoot
	err := r.db.WithContext(ctx).Order("id ASC").Find(&roots).Error
	return roots, err
}

func (r *mediaLibraryRepository) FindRootByID(ctx context.Context, id uint) (*domain.MediaLibraryRoot, error) {
	var root domain.MediaLibraryRoot
	if err := r.db.WithContext(ctx).First(&root, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrMediaRootNotFound
		}
		return nil, err
	}
	return &root, nil
}

func (r *mediaLibraryRepository) CreateRoot(ctx context.Context, root *domain.MediaLibraryRoot) error {
	return r.db.WithContext(ctx).Create(root).Error
}

func (r *mediaLibraryRepository) UpdateRoot(ctx context.Context, root *domain.MediaLibraryRoot) error {
	return r.db.WithContext(ctx).Save(root).Error
}

func (r *mediaLibraryRepository) FindRootState(ctx context.Context, rootID uint) (*domain.MediaLibraryRootState, error) {
	var state domain.MediaLibraryRootState
	if err := r.db.WithContext(ctx).First(&state, "root_id = ?", rootID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &state, nil
}

func (r *mediaLibraryRepository) UpsertRootState(ctx context.Context, state *domain.MediaLibraryRootState) error {
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "root_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"status", "code", "message", "filesystem", "mount_source", "retryable", "last_checked_at", "last_online_at", "updated_at"}),
		// A slow hard-mounted probe may finish after a newer manual or peer
		// probe. SQLite and PostgreSQL both expose the proposed row as excluded.
		Where: clause.Where{Exprs: []clause.Expression{clause.Expr{
			SQL: "media_library_root_states.last_checked_at IS NULL OR excluded.last_checked_at >= media_library_root_states.last_checked_at",
		}}},
	}).Create(state).Error
}

func (r *mediaLibraryRepository) FindMediaFileBySourceKey(ctx context.Context, sourceKey string) (*domain.MediaFile, error) {
	var mediaFile domain.MediaFile
	if err := r.db.WithContext(ctx).Where("source_key = ?", sourceKey).First(&mediaFile).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrMediaFileNotFound
		}
		return nil, err
	}
	return &mediaFile, nil
}

func (r *mediaLibraryRepository) ListMediaFilesByMusicID(ctx context.Context, musicID uint) ([]*domain.MediaFile, error) {
	var mediaFiles []*domain.MediaFile
	err := r.db.WithContext(ctx).Where("music_id = ?", musicID).
		Order("CASE availability WHEN 'online' THEN 0 WHEN 'unknown' THEN 1 ELSE 2 END, root_id ASC, id ASC").
		Find(&mediaFiles).Error
	return mediaFiles, err
}

func (r *mediaLibraryRepository) HasReadOnlyMediaFile(ctx context.Context, musicID uint) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&domain.MediaFile{}).
		Where("music_id = ? AND read_only = ?", musicID, true).
		Count(&count).Error
	return count > 0, err
}

func (r *mediaLibraryRepository) CreateMediaFile(ctx context.Context, mediaFile *domain.MediaFile) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(mediaFile).Error; err != nil {
			return err
		}
		if mediaFile.ReadOnly {
			return tx.Model(&domain.Music{}).Where("id = ?", mediaFile.MusicID).
				Update("source_read_only", true).Error
		}
		return nil
	})
}

func (r *mediaLibraryRepository) UpdateMediaFile(ctx context.Context, mediaFile *domain.MediaFile) error {
	return r.db.WithContext(ctx).Save(mediaFile).Error
}

func (r *mediaLibraryRepository) MarkRootFilesMissingBefore(ctx context.Context, rootID uint, observedAfter time.Time) error {
	return r.db.WithContext(ctx).Model(&domain.MediaFile{}).
		Where("root_id = ? AND (last_seen_at IS NULL OR last_seen_at < ?)", rootID, observedAfter).
		Where("availability <> ?", domain.MediaFileAvailabilityChanged).
		Update("availability", domain.MediaFileAvailabilityMissing).Error
}

func (r *mediaLibraryRepository) CreateMusicWithMediaFile(ctx context.Context, music *domain.Music, mediaFile *domain.MediaFile) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(music).Error; err != nil {
			return err
		}
		mediaFile.MusicID = music.ID
		return tx.Create(mediaFile).Error
	})
}

func (r *mediaLibraryRepository) ReplaceManagedMediaFile(ctx context.Context, music *domain.Music, mediaFile *domain.MediaFile) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var readOnlySourceCount int64
		if err := tx.Model(&domain.MediaFile{}).
			Where("music_id = ? AND read_only = ?", music.ID, true).
			Count(&readOnlySourceCount).Error; err != nil {
			return err
		}
		music.SourceReadOnly = mediaFile.ReadOnly || readOnlySourceCount > 0
		if err := tx.Save(music).Error; err != nil {
			return err
		}
		if err := tx.Unscoped().Where("music_id = ? AND root_id = ? AND source_key <> ?", music.ID, domain.ManagedMediaRootID, mediaFile.SourceKey).
			Delete(&domain.MediaFile{}).Error; err != nil {
			return err
		}
		mediaFile.MusicID = music.ID
		return tx.Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "source_key"}},
			DoUpdates: clause.AssignmentColumns([]string{
				"music_id", "root_id", "relative_path", "file_hash", "observed_file_hash", "file_size", "file_mod_time",
				"read_only", "availability", "content_revision", "last_seen_at", "last_verified_at", "updated_at", "deleted_at",
			}),
		}).Create(mediaFile).Error
	})
}

func (r *mediaLibraryRepository) DeleteRoot(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var activeScans int64
		if err := tx.Model(&domain.MediaScanJob{}).
			Where("root_id = ? AND status IN ?", id, activeMediaScanStatuses).
			Count(&activeScans).Error; err != nil {
			return err
		}
		if activeScans > 0 {
			return ErrMediaScanInProgress
		}
		var count int64
		if err := tx.Model(&domain.MediaFile{}).Where("root_id = ?", id).Count(&count).Error; err != nil {
			return err
		}
		if count > 0 {
			return ErrMediaRootInUse
		}
		result := tx.Delete(&domain.MediaLibraryRoot{}, id)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return ErrMediaRootNotFound
		}
		if err := tx.Delete(&domain.MediaLibraryRootState{}, "root_id = ?", id).Error; err != nil {
			return err
		}
		return nil
	})
}

func (r *mediaLibraryRepository) HasActiveScanForRoot(ctx context.Context, rootID uint) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&domain.MediaScanJob{}).
		Where("root_id = ? AND status IN ?", rootID, activeMediaScanStatuses).
		Count(&count).Error
	return count > 0, err
}

func (r *mediaLibraryRepository) CountMusicByRoot(ctx context.Context, rootID uint) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&domain.MediaFile{}).Where("root_id = ?", rootID).Distinct("music_id").Count(&count).Error
	return count, err
}

func (r *mediaLibraryRepository) CreateScanJob(ctx context.Context, job *domain.MediaScanJob) error {
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var count int64
		if err := tx.Model(&domain.MediaScanJob{}).
			Where("root_id = ? AND status IN ?", job.RootID, activeMediaScanStatuses).
			Count(&count).Error; err != nil {
			return err
		}
		if count > 0 {
			return ErrMediaScanInProgress
		}
		return tx.Create(job).Error
	})
	if err == nil || errors.Is(err, ErrMediaScanInProgress) {
		return err
	}
	// The partial unique index is the real cross-process arbiter. If another
	// instance won the race between COUNT and INSERT, expose a stable domain
	// error instead of a driver-specific uniqueness message.
	active, checkErr := r.HasActiveScanForRoot(ctx, job.RootID)
	if checkErr == nil && active {
		return ErrMediaScanInProgress
	}
	return err
}

func (r *mediaLibraryRepository) ClaimNextScanJob(ctx context.Context, owner string, leaseDuration time.Duration) (*domain.MediaScanJob, bool, error) {
	var claimed *domain.MediaScanJob
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var job domain.MediaScanJob
		now := time.Now().UTC()
		if err := tx.Where("status = ? OR (status = ? AND (next_attempt_at IS NULL OR next_attempt_at <= ?))",
			domain.MediaScanStatusPending, domain.MediaScanStatusRetryWait, now).
			Order("id ASC").First(&job).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil
			}
			return err
		}
		leaseExpiresAt := now.Add(leaseDuration)
		result := tx.Model(&domain.MediaScanJob{}).
			Where("id = ? AND status = ?", job.ID, job.Status).
			Updates(map[string]interface{}{
				"status":            domain.MediaScanStatusRunning,
				"started_at":        gorm.Expr("COALESCE(started_at, ?)", now),
				"heartbeat_at":      now,
				"attempt":           gorm.Expr("attempt + 1"),
				"next_attempt_at":   nil,
				"finished_at":       nil,
				"failure_code":      "",
				"failure_retryable": false,
				"error_summary":     "",
				"lease_owner":       owner,
				"lease_expires_at":  leaseExpiresAt,
				"lease_generation":  gorm.Expr("lease_generation + 1"),
				"discovered_count":  0,
				"processed_count":   0,
				"imported_count":    0,
				"existing_count":    0,
				"duplicate_count":   0,
				"skipped_count":     0,
				"warning_count":     0,
				"failed_count":      0,
			})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return nil
		}
		if err := tx.Where("job_id = ?", job.ID).Delete(&domain.MediaScanIssue{}).Error; err != nil {
			return err
		}
		if err := tx.First(&job, job.ID).Error; err != nil {
			return err
		}
		claimed = &job
		return nil
	})
	return claimed, claimed != nil, err
}

func (r *mediaLibraryRepository) FindScanJob(ctx context.Context, id uint) (*domain.MediaScanJob, error) {
	var job domain.MediaScanJob
	if err := r.db.WithContext(ctx).First(&job, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrMediaScanNotFound
		}
		return nil, err
	}
	return &job, nil
}

func (r *mediaLibraryRepository) ListScanJobs(ctx context.Context, rootID *uint, page, pageSize int) ([]*domain.MediaScanJob, int64, error) {
	query := r.db.WithContext(ctx).Model(&domain.MediaScanJob{})
	if rootID != nil {
		query = query.Where("root_id = ?", *rootID)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var jobs []*domain.MediaScanJob
	err := query.Order("id DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&jobs).Error
	return jobs, total, err
}

// UpdateScanJob intentionally excludes CancelRequested. A cancellation may be
// set concurrently by the HTTP handler and must not be lost when the worker
// flushes progress from an older in-memory copy.
func (r *mediaLibraryRepository) UpdateScanJob(ctx context.Context, job *domain.MediaScanJob) error {
	query := r.db.WithContext(ctx).Model(&domain.MediaScanJob{}).Where("id = ?", job.ID)
	leaseOwner := job.LeaseOwner
	if leaseOwner != "" {
		query = query.Where("lease_owner = ? AND lease_generation = ?", leaseOwner, job.LeaseGeneration)
	}
	updates := map[string]interface{}{
		"status":            job.Status,
		"discovered_count":  job.DiscoveredCount,
		"processed_count":   job.ProcessedCount,
		"imported_count":    job.ImportedCount,
		"existing_count":    job.ExistingCount,
		"duplicate_count":   job.DuplicateCount,
		"skipped_count":     job.SkippedCount,
		"warning_count":     job.WarningCount,
		"failed_count":      job.FailedCount,
		"started_at":        job.StartedAt,
		"heartbeat_at":      job.HeartbeatAt,
		"finished_at":       job.FinishedAt,
		"error_summary":     job.ErrorSummary,
		"attempt":           job.Attempt,
		"next_attempt_at":   job.NextAttemptAt,
		"failure_code":      job.FailureCode,
		"failure_retryable": job.FailureRetryable,
		"lease_expires_at":  job.LeaseExpiresAt,
	}
	if job.Status != domain.MediaScanStatusRunning {
		updates["lease_owner"] = ""
		updates["lease_expires_at"] = nil
	}
	result := query.Updates(updates)
	if result.Error != nil {
		return result.Error
	}
	if leaseOwner != "" && result.RowsAffected == 0 {
		return ErrMediaScanLeaseLost
	}
	if job.Status != domain.MediaScanStatusRunning {
		job.LeaseOwner = ""
		job.LeaseExpiresAt = nil
	}
	return nil
}

func (r *mediaLibraryRepository) RequestScanCancellation(ctx context.Context, id uint) (*domain.MediaScanJob, error) {
	job, err := r.FindScanJob(ctx, id)
	if err != nil {
		return nil, err
	}
	switch job.Status {
	case domain.MediaScanStatusPending, domain.MediaScanStatusRetryWait:
		now := time.Now().UTC()
		result := r.db.WithContext(ctx).Model(&domain.MediaScanJob{}).Where("id = ? AND status IN ?", id, []string{domain.MediaScanStatusPending, domain.MediaScanStatusRetryWait}).
			Updates(map[string]interface{}{"status": domain.MediaScanStatusCancelled, "cancel_requested": true, "finished_at": now})
		if result.Error != nil {
			return nil, result.Error
		}
		if result.RowsAffected == 0 {
			if err := r.db.WithContext(ctx).Model(&domain.MediaScanJob{}).
				Where("id = ? AND status = ?", id, domain.MediaScanStatusRunning).
				Update("cancel_requested", true).Error; err != nil {
				return nil, err
			}
		}
	case domain.MediaScanStatusRunning:
		if err := r.db.WithContext(ctx).Model(&domain.MediaScanJob{}).
			Where("id = ? AND status = ?", id, domain.MediaScanStatusRunning).
			Update("cancel_requested", true).Error; err != nil {
			return nil, err
		}
	}
	return r.FindScanJob(ctx, id)
}

func (r *mediaLibraryRepository) IsScanCancellationRequested(ctx context.Context, id uint) (bool, error) {
	var job struct {
		CancelRequested bool
		Status          string
	}
	if err := r.db.WithContext(ctx).Model(&domain.MediaScanJob{}).Select("cancel_requested", "status").First(&job, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return false, ErrMediaScanNotFound
		}
		return false, err
	}
	return job.CancelRequested || job.Status == domain.MediaScanStatusCancelled, nil
}

func (r *mediaLibraryRepository) RecoverExpiredScanJobs(ctx context.Context) error {
	now := time.Now().UTC()
	return r.db.WithContext(ctx).Model(&domain.MediaScanJob{}).
		Where("status = ? AND (lease_expires_at IS NULL OR lease_expires_at <= ?)", domain.MediaScanStatusRunning, now).
		Updates(map[string]interface{}{
			"status":            domain.MediaScanStatusRetryWait,
			"next_attempt_at":   now,
			"heartbeat_at":      now,
			"failure_code":      "worker_lease_expired",
			"failure_retryable": true,
			"lease_owner":       "",
			"lease_expires_at":  nil,
			"error_summary":     "scan worker lease expired; the job will resume incrementally",
		}).Error
}

func (r *mediaLibraryRepository) CreateScanIssue(ctx context.Context, issue *domain.MediaScanIssue) error {
	return r.db.WithContext(ctx).Create(issue).Error
}

func (r *mediaLibraryRepository) ListScanIssues(ctx context.Context, jobID uint, limit int) ([]*domain.MediaScanIssue, error) {
	var issues []*domain.MediaScanIssue
	err := r.db.WithContext(ctx).Where("job_id = ?", jobID).Order("id ASC").Limit(limit).Find(&issues).Error
	return issues, err
}
