package service

import (
	"context"
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
	"time"

	"github.com/kyangconn/music-online-go/internal/domain"
	"github.com/kyangconn/music-online-go/internal/mediametadata"
	pklog "github.com/kyangconn/music-online-go/internal/pkg/log"
	"github.com/kyangconn/music-online-go/internal/repository"
)

const (
	mediaScanPollInterval = time.Second
	// Lease timing is an internal consistency mechanism, not a deployment
	// tuning knob. Long file hashes refresh it periodically.
	mediaScanLeaseDuration = 15 * time.Minute
	mediaScanIssueLimit    = 200
	unknownArtist          = "Unknown Artist"
)

type scanFileDisposition uint8

const (
	scanFileSkipped scanFileDisposition = iota + 1
	scanFileExisting
	scanFileDuplicate
	scanFileImported
	scanFileFailed
)

type scanFileIssue struct {
	severity string
	code     string
	message  string
}

// scanFileResult is the boundary between per-file persistence and scan-job
// accounting. Every processed file has one disposition, while warnings and a
// transient storage signal remain orthogonal.
type scanFileResult struct {
	disposition             scanFileDisposition
	issues                  []scanFileIssue
	transientStorageFailure bool
	stopErr                 error
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
			result := scanFileResult{
				disposition:             scanFileFailed,
				transientStorageFailure: isRetryableMediaStorageError(walkErr),
				issues: []scanFileIssue{{
					severity: "error", code: "directory_unavailable",
					message: fmt.Sprintf("directory entry could not be read: %v", walkErr),
				}},
			}
			transientStorageFailure = transientStorageFailure || result.transientStorageFailure
			if err := s.applyScanFileResult(workerCtx, job, safeRelativePath(root.Path, path), result); err != nil {
				return err
			}
			if entry != nil && entry.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if entry.IsDir() || entry.Type()&os.ModeSymlink != 0 || !isScannableAudioName(entry.Name()) {
			return nil
		}
		job.DiscoveredCount++
		result := s.processScanFile(workerCtx, job, root, path, entry)
		job.ProcessedCount++
		transientStorageFailure = transientStorageFailure || result.transientStorageFailure
		if err := s.applyScanFileResult(workerCtx, job, safeRelativePath(root.Path, path), result); err != nil {
			return err
		}
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

func (s *mediaLibraryService) processScanFile(ctx context.Context, job *domain.MediaScanJob, root *resolvedMediaRoot, path string, entry fs.DirEntry) scanFileResult {
	info, err := entry.Info()
	if err != nil {
		return scanFileResult{
			disposition: scanFileFailed, transientStorageFailure: isRetryableMediaStorageError(err),
			issues: []scanFileIssue{{severity: "error", code: "file_unavailable", message: "file information could not be read"}},
		}
	}
	if !info.Mode().IsRegular() {
		return scanFileResult{disposition: scanFileSkipped}
	}
	relative := safeRelativePath(root.Path, path)
	sourceKey := mediaSourceKey(root.ID, relative, root.PathSemantics)
	existingFile, err := s.repo.FindMediaFileBySourceKey(ctx, sourceKey)
	if err != nil && !errors.Is(err, repository.ErrMediaFileNotFound) {
		return scanFileResult{
			disposition: scanFileFailed,
			issues:      []scanFileIssue{{severity: "error", code: "database_read_failed", message: "existing track state could not be checked"}},
		}
	}
	now := time.Now().UTC()
	markExistingSeen := func(availability string) error {
		if existingFile == nil {
			return nil
		}
		existingFile.LastSeenAt = &now
		if availability != "" {
			existingFile.Availability = availability
		}
		return s.repo.UpdateMediaFile(ctx, existingFile)
	}
	if info.Size() <= 0 {
		if err := markExistingSeen(domain.MediaFileAvailabilityChanged); err != nil {
			return databaseRefreshFailure()
		}
		return scanFileResult{
			disposition: scanFileSkipped,
			issues:      []scanFileIssue{{severity: "warning", code: "empty_file", message: "empty audio file was skipped"}},
		}
	}
	maxBytes := int64(s.config.Scanner.MaxFileSizeMB) * 1024 * 1024
	if info.Size() > maxBytes {
		if err := markExistingSeen(domain.MediaFileAvailabilityChanged); err != nil {
			return databaseRefreshFailure()
		}
		return scanFileResult{
			disposition: scanFileSkipped,
			issues:      []scanFileIssue{{severity: "warning", code: "file_too_large", message: "audio file exceeds the configured scanner size limit"}},
		}
	}
	minimumAge := time.Duration(s.config.Scanner.MinFileAgeSeconds) * time.Second
	if minimumAge > 0 && time.Since(info.ModTime()) < minimumAge {
		if err := markExistingSeen(""); err != nil {
			return databaseRefreshFailure()
		}
		return scanFileResult{disposition: scanFileSkipped}
	}
	if existingFile != nil && existingFile.FileSize == info.Size() && sameFileTimestamp(existingFile.FileModTime, info.ModTime()) &&
		!sourceNeedsHashRecheck(existingFile.LastVerifiedAt, s.config.Scanner.HashRecheckHours, now) {
		if existingFile.ObservedFileHash == "" {
			existingFile.ObservedFileHash = existingFile.FileHash
		}
		existingFile.Availability = domain.MediaFileAvailabilityOnline
		existingFile.LastSeenAt = &now
		if err := s.repo.UpdateMediaFile(ctx, existingFile); err != nil {
			return databaseRefreshFailure()
		}
		return scanFileResult{disposition: scanFileExisting}
	}

	if err := validateScannedAudio(path); err != nil {
		if errors.Is(err, ErrInvalidMediaFile) {
			if existingFile != nil {
				existingFile.Availability = domain.MediaFileAvailabilityChanged
				existingFile.LastSeenAt = &now
				_ = s.repo.UpdateMediaFile(ctx, existingFile)
			}
			return scanFileResult{
				disposition: scanFileSkipped,
				issues:      []scanFileIssue{{severity: "warning", code: "unsupported_content", message: "file does not contain a supported audio signature"}},
			}
		}
		if existingFile != nil {
			existingFile.Availability = domain.MediaFileAvailabilityOffline
			_ = s.repo.UpdateMediaFile(ctx, existingFile)
		}
		return scanFileResult{
			disposition: scanFileFailed, transientStorageFailure: isRetryableMediaStorageError(err),
			issues: []scanFileIssue{{severity: "error", code: "file_unavailable", message: fmt.Sprintf("audio header could not be read: %v", err)}},
		}
	}
	fileHash, err := hashScannedFile(ctx, path, func() error { return s.renewScanLease(ctx, job) })
	if err != nil {
		if errors.Is(err, repository.ErrMediaScanLeaseLost) || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return scanFileResult{stopErr: err}
		}
		if existingFile != nil {
			existingFile.Availability = domain.MediaFileAvailabilityOffline
			_ = s.repo.UpdateMediaFile(ctx, existingFile)
		}
		return scanFileResult{
			disposition: scanFileFailed, transientStorageFailure: isRetryableMediaStorageError(err),
			issues: []scanFileIssue{{severity: "error", code: "file_read_failed", message: "audio file could not be read completely"}},
		}
	}
	if stable, err := sourceFileStateUnchanged(path, info); err != nil || !stable {
		if existingFile != nil {
			existingFile.Availability = domain.MediaFileAvailabilityChanged
			existingFile.LastSeenAt = &now
			_ = s.repo.UpdateMediaFile(ctx, existingFile)
		}
		return scanFileResult{
			disposition: scanFileSkipped, transientStorageFailure: isRetryableMediaStorageError(err),
			issues: []scanFileIssue{{severity: "warning", code: "source_busy", message: "source changed while it was being read; retry after the file is stable"}},
		}
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
				return databaseRefreshFailure()
			}
			return scanFileResult{disposition: scanFileExisting}
		}
		existingFile.ObservedFileHash = fileHash
		existingFile.FileSize = info.Size()
		existingFile.FileModTime = &modTime
		existingFile.Availability = domain.MediaFileAvailabilityChanged
		existingFile.LastSeenAt = &now
		existingFile.LastVerifiedAt = &now
		existingFile.ContentRevision++
		if err := s.repo.UpdateMediaFile(ctx, existingFile); err != nil {
			return scanFileResult{
				disposition: scanFileFailed,
				issues:      []scanFileIssue{{severity: "error", code: "database_write_failed", message: "changed source state could not be recorded"}},
			}
		}
		return scanFileResult{
			disposition: scanFileSkipped,
			issues:      []scanFileIssue{{severity: "warning", code: "source_changed", message: "source content changed; the old logical track and analysis identity were preserved for manual review"}},
		}
	}

	exact, err := s.musicRepo.FindByFileHash(ctx, fileHash)
	if err != nil && !errors.Is(err, repository.ErrMusicNotFound) {
		return scanFileResult{
			disposition: scanFileFailed,
			issues:      []scanFileIssue{{severity: "error", code: "database_read_failed", message: "duplicate state could not be checked"}},
		}
	}
	if exact != nil {
		mediaFile := newScannedMediaFile(exact.ID, root, relative, sourceKey, fileHash, info, now)
		if err := s.repo.CreateMediaFile(ctx, mediaFile); err != nil {
			return scanFileResult{
				disposition: scanFileFailed,
				issues:      []scanFileIssue{{severity: "error", code: "database_write_failed", message: "duplicate source could not be linked to the logical track"}},
			}
		}
		// A duplicate becomes a playback fallback and can share future M5
		// analysis artifacts through its content hash.
		return scanFileResult{disposition: scanFileDuplicate}
	}

	metadata, picture, metadataErr := mediametadata.Read(path, int64(s.config.Scanner.MaxTagSizeMB)*1024*1024, s.maxCoverBytes)
	issues := make([]scanFileIssue, 0, 2)
	if metadataErr != nil {
		if isRetryableMediaStorageError(metadataErr) {
			return scanFileResult{
				disposition: scanFileFailed, transientStorageFailure: true,
				issues: []scanFileIssue{{severity: "error", code: "storage_read_interrupted", message: "network storage interrupted metadata reading; this source will be retried"}},
			}
		}
		issues = append(issues, scanFileIssue{
			severity: "warning", code: "metadata_unreadable",
			message: fmt.Sprintf("some embedded tags could not be read; safe filename fallbacks were used: %v", metadataErr),
		})
	}
	if strings.TrimSpace(metadata.Title) == "" {
		metadata.Title = strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	}
	if strings.TrimSpace(metadata.Artist) == "" {
		metadata.Artist = unknownArtist
	}
	if stable, err := sourceFileStateUnchanged(path, info); err != nil || !stable {
		issues = append(issues, scanFileIssue{
			severity: "warning", code: "source_busy",
			message: "source changed while metadata was being read; retry after the file is stable",
		})
		return scanFileResult{
			disposition: scanFileSkipped, issues: issues,
			transientStorageFailure: isRetryableMediaStorageError(err),
		}
	}
	music := &domain.Music{
		Path: path, FileHash: fileHash, Type: domain.MusicTypeSingle, UserID: 0,
		MediaRootID: root.ID, MediaRelativePath: relative, MediaSourceKey: &sourceKey,
		SourceFileSize: info.Size(), SourceFileModTime: &modTime, SourceReadOnly: root.ReadOnly,
	}
	if err := applyCreateMusicMetadata(music, metadata); err != nil {
		issues = append(issues, scanFileIssue{severity: "error", code: "invalid_metadata", message: "embedded metadata could not be normalized"})
		return scanFileResult{disposition: scanFileFailed, issues: issues}
	}
	mediaFile := newScannedMediaFile(0, root, relative, sourceKey, fileHash, info, now)
	if err := s.repo.CreateMusicWithMediaFile(ctx, music, mediaFile); err != nil {
		issues = append(issues, scanFileIssue{severity: "error", code: "database_write_failed", message: "track could not be added to the library"})
		return scanFileResult{disposition: scanFileFailed, issues: issues}
	}
	if picture != nil {
		coverPath, err := mediametadata.SaveCover(music.ID, picture, s.managedPath, s.maxCoverBytes)
		if err != nil {
			issues = append(issues, scanFileIssue{severity: "warning", code: "cover_extract_failed", message: fmt.Sprintf("embedded cover could not be copied to managed storage: %v", err)})
		} else if coverPath != "" {
			music.Img = coverPath
			if err := s.musicRepo.Update(ctx, music); err != nil {
				cleanupUploadedFiles([]string{coverPath})
				issues = append(issues, scanFileIssue{severity: "warning", code: "cover_store_failed", message: "embedded cover was not attached to the track"})
			}
		}
	}
	scheduleCtx, cancelSchedule := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	if err := s.analyzer.ScheduleContentAnalysis(scheduleCtx, music.ID, job.RequestedBy); err != nil {
		// Import remains successful even when derived analysis is disabled,
		// backpressured, or temporarily unavailable.
		pklog.Warnf("Music %d was imported but analysis could not be queued: %v", music.ID, err)
	}
	cancelSchedule()
	return scanFileResult{disposition: scanFileImported, issues: issues}
}

func databaseRefreshFailure() scanFileResult {
	return scanFileResult{
		disposition: scanFileFailed,
		issues:      []scanFileIssue{{severity: "error", code: "database_write_failed", message: "source file state could not be refreshed"}},
	}
}

func (s *mediaLibraryService) applyScanFileResult(ctx context.Context, job *domain.MediaScanJob, relative string, result scanFileResult) error {
	switch result.disposition {
	case scanFileSkipped:
		job.SkippedCount++
	case scanFileExisting:
		job.ExistingCount++
	case scanFileDuplicate:
		job.DuplicateCount++
	case scanFileImported:
		job.ImportedCount++
	case scanFileFailed:
		job.FailedCount++
	}
	for _, issue := range result.issues {
		if issue.severity == "warning" {
			job.WarningCount++
		}
		if job.WarningCount+job.FailedCount > mediaScanIssueLimit {
			continue
		}
		row := &domain.MediaScanIssue{
			JobID: job.ID, RelativePath: truncateText(relative, 1000), Severity: issue.severity,
			Code: issue.code, Message: truncateText(issue.message, 500),
		}
		if err := s.repo.CreateScanIssue(ctx, row); err != nil {
			pklog.Errorf("Failed to record issue for media scan job %d: %v", job.ID, err)
		}
	}
	return result.stopErr
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
	// #nosec G304 -- path comes from a configured administrator-owned media root.
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
	// #nosec G304 -- path comes from a configured administrator-owned media root.
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
