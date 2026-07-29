package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/kyangconn/music-online-go/internal/config"
	"github.com/kyangconn/music-online-go/internal/domain"
	pklog "github.com/kyangconn/music-online-go/internal/pkg/log"
	"github.com/kyangconn/music-online-go/internal/repository"
)

var (
	ErrMusicAnalysisDisabled = errors.New("music analysis is disabled")
	ErrAudioAnalyzerDisabled = errors.New("audio analyzer is disabled")
	ErrAnalysisSourceMissing = errors.New("music has no analyzable audio source")
	errAnalysisContentStale  = errors.New("music analysis content is stale")
	errAnalysisCancelled     = errors.New("music analysis was cancelled")
)

const (
	analysisPollInterval      = time.Second
	analysisHeartbeatInterval = time.Second
	analysisRecoveryInterval  = 30 * time.Second
	analysisMetadataTimeout   = 30 * time.Second
	analysisFinalizeTimeout   = 5 * time.Second
	analysisBackfillBatchSize = 200
)

// MusicAnalysisScheduler is intentionally small so upload and import paths
// only know how to append durable work after their own transaction succeeds.
type MusicAnalysisScheduler interface {
	ScheduleContentAnalysis(ctx context.Context, musicID, requestedBy uint) error
}

type MusicAnalysisService interface {
	MusicAnalysisScheduler
	ScheduleMusic(ctx context.Context, musicID, requestedBy uint, request domain.AnalysisEnqueueRequest) (*domain.AnalysisScheduleResponse, error)
	Backfill(ctx context.Context, requestedBy uint, request domain.AnalysisBackfillRequest) (*domain.AnalysisBackfillResponse, error)
	ListJobs(ctx context.Context, params domain.AnalysisJobListParams) ([]*domain.MusicAnalysisJob, int64, error)
	GetJob(ctx context.Context, id uint) (*domain.MusicAnalysisJob, error)
	CancelJob(ctx context.Context, id uint) (*domain.MusicAnalysisJob, error)
	Metrics(ctx context.Context) (*domain.AnalysisQueueMetrics, error)
	Start(ctx context.Context) error
	Shutdown(ctx context.Context) error
}

type musicAnalysisService struct {
	repo         repository.MusicAnalysisRepository
	presetRepo   repository.PresetRepository
	pathResolver MediaPathResolver
	config       config.ClassificationConfig
	analyzer     audioAnalyzer
	workerID     string

	mu      sync.Mutex
	cancel  context.CancelFunc
	started bool
	wg      sync.WaitGroup
}

func NewMusicAnalysisService(
	repo repository.MusicAnalysisRepository,
	presetRepo repository.PresetRepository,
	pathResolver MediaPathResolver,
	cfg config.ClassificationConfig,
) MusicAnalysisService {
	var analyzer audioAnalyzer
	if cfg.Analyzer.Mode == "http" {
		analyzer = newHTTPAudioAnalyzer(cfg.Analyzer)
	}
	return newMusicAnalysisServiceWithAnalyzer(repo, presetRepo, pathResolver, cfg, analyzer)
}

func newMusicAnalysisServiceWithAnalyzer(
	repo repository.MusicAnalysisRepository,
	presetRepo repository.PresetRepository,
	pathResolver MediaPathResolver,
	cfg config.ClassificationConfig,
	analyzer audioAnalyzer,
) MusicAnalysisService {
	if cfg.Analyzer.Concurrency <= 0 {
		cfg.Analyzer.Concurrency = 1
	}
	if cfg.Analyzer.QueueLimit <= 0 {
		cfg.Analyzer.QueueLimit = 1000
	}
	if cfg.Analyzer.TimeoutSeconds <= 0 {
		cfg.Analyzer.TimeoutSeconds = 300
	}
	if cfg.Analyzer.MaxFileSizeMB <= 0 {
		cfg.Analyzer.MaxFileSizeMB = 2048
	}
	if cfg.Analyzer.MaxDurationSeconds <= 0 {
		cfg.Analyzer.MaxDurationSeconds = 1800
	}
	if cfg.Analyzer.RetryMaxAttempts <= 0 {
		cfg.Analyzer.RetryMaxAttempts = 3
	}
	if cfg.Analyzer.RetryInitialSeconds <= 0 {
		cfg.Analyzer.RetryInitialSeconds = 30
	}
	if cfg.Analyzer.RetryMaxSeconds < cfg.Analyzer.RetryInitialSeconds {
		cfg.Analyzer.RetryMaxSeconds = 900
	}
	return &musicAnalysisService{
		repo: repo, presetRepo: presetRepo, pathResolver: pathResolver,
		config: cfg, analyzer: analyzer, workerID: newAnalysisWorkerID(),
	}
}

func (service *musicAnalysisService) ScheduleContentAnalysis(ctx context.Context, musicID, requestedBy uint) error {
	if service.repo == nil {
		return nil
	}
	candidate, err := service.repo.FindCandidate(ctx, musicID)
	if err != nil {
		return err
	}
	if err := service.repo.MarkSupersededAudio(ctx, musicID, candidate.FileHash); err != nil {
		return fmt.Errorf("mark superseded music analyses: %w", err)
	}
	if !service.config.Enabled || !service.config.AnalyzeOnUpload || service.analyzer == nil {
		return nil
	}
	_, _, err = service.enqueueAudio(ctx, candidate, requestedBy, false)
	if errors.Is(err, ErrAnalysisSourceMissing) {
		return nil
	}
	return err
}

func (service *musicAnalysisService) ScheduleMusic(
	ctx context.Context,
	musicID, requestedBy uint,
	request domain.AnalysisEnqueueRequest,
) (*domain.AnalysisScheduleResponse, error) {
	if !service.config.Enabled {
		return nil, ErrMusicAnalysisDisabled
	}
	if request.IncludeAudio && service.analyzer == nil {
		return nil, ErrAudioAnalyzerDisabled
	}
	candidate, err := service.repo.FindCandidate(ctx, musicID)
	if err != nil {
		return nil, err
	}
	response := &domain.AnalysisScheduleResponse{}
	metadataJob, queued, err := service.enqueueMetadata(ctx, candidate, requestedBy, request.Force)
	if err != nil {
		return nil, err
	}
	response.MetadataJob = metadataJob
	if !queued {
		response.Reused++
	}
	if request.IncludeAudio {
		if err := service.repo.MarkSupersededAudio(ctx, musicID, candidate.FileHash); err != nil {
			return nil, err
		}
		audioJob, audioQueued, err := service.enqueueAudio(ctx, candidate, requestedBy, request.Force)
		if errors.Is(err, ErrAnalysisSourceMissing) {
			response.Skipped++
			return response, nil
		}
		if err != nil {
			return nil, err
		}
		response.AudioJob = audioJob
		if !audioQueued {
			response.Reused++
		}
	}
	return response, nil
}

func (service *musicAnalysisService) Backfill(
	ctx context.Context,
	requestedBy uint,
	request domain.AnalysisBackfillRequest,
) (*domain.AnalysisBackfillResponse, error) {
	if !service.config.Enabled {
		return nil, ErrMusicAnalysisDisabled
	}
	if request.IncludeAudio && service.analyzer == nil {
		return nil, ErrAudioAnalyzerDisabled
	}
	response := &domain.AnalysisBackfillResponse{}
	var afterID uint
	for {
		candidates, err := service.repo.ListCandidates(ctx, afterID, analysisBackfillBatchSize)
		if err != nil {
			return nil, err
		}
		for _, candidate := range candidates {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			afterID = candidate.Music.ID
			response.Visited++
			_, queued, err := service.enqueueMetadata(ctx, candidate, requestedBy, false)
			if errors.Is(err, repository.ErrAnalysisQueueFull) {
				response.QueueRejected++
				return response, nil
			}
			if err != nil {
				return nil, err
			}
			if queued {
				response.RulesQueued++
			} else {
				response.Reused++
			}
			if !request.IncludeAudio {
				continue
			}
			_, audioQueued, err := service.enqueueAudio(ctx, candidate, requestedBy, false)
			switch {
			case errors.Is(err, ErrAnalysisSourceMissing):
				response.Skipped++
			case errors.Is(err, repository.ErrAnalysisQueueFull):
				response.QueueRejected++
				return response, nil
			case err != nil:
				return nil, err
			case audioQueued:
				response.AudioQueued++
			default:
				response.Reused++
			}
		}
		if len(candidates) < analysisBackfillBatchSize {
			return response, nil
		}
	}
}

func (service *musicAnalysisService) enqueueMetadata(
	ctx context.Context,
	candidate *domain.AnalysisMusicCandidate,
	requestedBy uint,
	force bool,
) (*domain.MusicAnalysisJob, bool, error) {
	job := &domain.MusicAnalysisJob{
		Kind: domain.AnalysisJobKindMetadataRules, MusicID: candidate.Music.ID,
		RequestedBy: requestedBy, MetadataRevision: candidate.Music.MetadataRevision,
		RuleVersion: domain.PresetRuleVersion, Status: domain.AnalysisStatusPending,
		MaxAttempts: service.config.Analyzer.RetryMaxAttempts,
	}
	job.IdempotencyKey = analysisIdempotencyKey(
		job.Kind, strconv.FormatUint(uint64(job.MusicID), 10),
		strconv.FormatUint(job.MetadataRevision, 10), job.RuleVersion,
	)
	return service.repo.Enqueue(ctx, job, force, service.config.Analyzer.QueueLimit)
}

func (service *musicAnalysisService) enqueueAudio(
	ctx context.Context,
	candidate *domain.AnalysisMusicCandidate,
	requestedBy uint,
	force bool,
) (*domain.MusicAnalysisJob, bool, error) {
	if service.analyzer == nil {
		return nil, false, ErrAudioAnalyzerDisabled
	}
	if !validSHA256(candidate.FileHash) {
		return nil, false, ErrAnalysisSourceMissing
	}
	job := &domain.MusicAnalysisJob{
		Kind: domain.AnalysisJobKindAudio, MusicID: candidate.Music.ID,
		MediaFileID: candidate.MediaFileID, RequestedBy: requestedBy,
		FileHash: candidate.FileHash, ContentRevision: candidate.ContentRevision,
		MetadataRevision: candidate.Music.MetadataRevision,
		AnalyzerID:       service.config.Analyzer.ID, AnalyzerVersion: service.config.Analyzer.Version,
		ModelVersion: service.config.Analyzer.ModelVersion, Status: domain.AnalysisStatusPending,
		MaxAttempts: service.config.Analyzer.RetryMaxAttempts,
	}
	job.IdempotencyKey = analysisIdempotencyKey(
		job.Kind, strconv.FormatUint(uint64(job.MusicID), 10), job.FileHash,
		strconv.FormatUint(job.ContentRevision, 10), job.AnalyzerID, job.AnalyzerVersion, job.ModelVersion,
	)
	return service.repo.Enqueue(ctx, job, force, service.config.Analyzer.QueueLimit)
}

func (service *musicAnalysisService) ListJobs(ctx context.Context, params domain.AnalysisJobListParams) ([]*domain.MusicAnalysisJob, int64, error) {
	if params.Kind != "" && !domain.IsAnalysisJobKind(params.Kind) {
		return nil, 0, errors.New("invalid analysis job kind")
	}
	if params.Status != "" && !domain.IsAnalysisJobStatus(params.Status) {
		return nil, 0, errors.New("invalid analysis job status")
	}
	return service.repo.ListJobs(ctx, params)
}

func (service *musicAnalysisService) GetJob(ctx context.Context, id uint) (*domain.MusicAnalysisJob, error) {
	return service.repo.FindJob(ctx, id)
}

func (service *musicAnalysisService) CancelJob(ctx context.Context, id uint) (*domain.MusicAnalysisJob, error) {
	return service.repo.RequestCancellation(ctx, id)
}

func (service *musicAnalysisService) Metrics(ctx context.Context) (*domain.AnalysisQueueMetrics, error) {
	return service.repo.Metrics(ctx)
}

func (service *musicAnalysisService) Start(parent context.Context) error {
	service.mu.Lock()
	defer service.mu.Unlock()
	if service.started {
		return nil
	}
	if service.repo == nil {
		return errors.New("music analysis repository is nil")
	}
	if err := service.repo.RecoverExpired(parent); err != nil {
		return fmt.Errorf("recover interrupted music analyses: %w", err)
	}
	service.started = true
	ctx, cancel := context.WithCancel(parent)
	service.cancel = cancel
	if !service.config.Enabled {
		return nil
	}
	workers := service.config.Analyzer.Concurrency
	if service.analyzer == nil {
		workers = 1
	}
	for index := 0; index < workers; index++ {
		service.wg.Add(1)
		go service.worker(ctx, index)
	}
	return nil
}

func (service *musicAnalysisService) Shutdown(ctx context.Context) error {
	service.mu.Lock()
	if service.cancel != nil {
		service.cancel()
	}
	service.mu.Unlock()
	done := make(chan struct{})
	go func() {
		service.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (service *musicAnalysisService) worker(ctx context.Context, index int) {
	defer service.wg.Done()
	workerOwner := fmt.Sprintf("%s-%d", service.workerID, index)
	ticker := time.NewTicker(analysisPollInterval)
	defer ticker.Stop()
	lastRecovery := time.Time{}
	for {
		if ctx.Err() != nil {
			return
		}
		if lastRecovery.IsZero() || time.Since(lastRecovery) >= analysisRecoveryInterval {
			if err := service.repo.RecoverExpired(ctx); err != nil && ctx.Err() == nil {
				pklog.Errorf("Failed to recover expired music analysis jobs: %v", err)
			}
			lastRecovery = time.Now()
		}
		job, found, err := service.repo.ClaimNext(ctx, workerOwner, service.leaseDuration())
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			pklog.Errorf("Failed to claim music analysis job: %v", err)
		} else if found {
			service.runJob(ctx, job)
			continue
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (service *musicAnalysisService) runJob(workerCtx context.Context, job *domain.MusicAnalysisJob) {
	timeout := analysisMetadataTimeout
	if job.Kind == domain.AnalysisJobKindAudio {
		timeout = time.Duration(service.config.Analyzer.TimeoutSeconds) * time.Second
	}
	runCtx, cancelRun := context.WithTimeout(workerCtx, timeout)
	defer cancelRun()

	controlDone := make(chan struct{})
	controlExited := make(chan struct{})
	controlError := make(chan error, 1)
	leaseJob := *job
	go func() {
		defer close(controlExited)
		ticker := time.NewTicker(analysisHeartbeatInterval)
		defer ticker.Stop()
		for {
			select {
			case <-controlDone:
				return
			case <-runCtx.Done():
				return
			case <-ticker.C:
				cancelled, err := service.repo.Heartbeat(workerCtx, &leaseJob, service.leaseDuration())
				if err != nil {
					select {
					case controlError <- err:
					default:
					}
					cancelRun()
					return
				}
				if cancelled {
					select {
					case controlError <- errAnalysisCancelled:
					default:
					}
					cancelRun()
					return
				}
			}
		}
	}()

	startedAt := time.Now()
	err := service.processJob(runCtx, job)
	close(controlDone)
	<-controlExited
	var controlCause error
	select {
	case controlErr := <-controlError:
		controlCause = controlErr
		if err == nil || errors.Is(err, context.Canceled) {
			err = controlErr
		}
	default:
	}
	job.ProcessingMS = time.Since(startedAt).Milliseconds()
	if workerCtx.Err() != nil {
		err = newAnalyzerFailure("worker_shutdown", true, "analysis worker stopped before completion", workerCtx.Err())
	} else if errors.Is(controlCause, errAnalysisCancelled) {
		err = errAnalysisCancelled
	} else if errors.Is(runCtx.Err(), context.DeadlineExceeded) {
		err = newAnalyzerFailure("analysis_timeout", true, "analysis exceeded its configured timeout", runCtx.Err())
	}
	service.finishAttempt(job, err)
}

func (service *musicAnalysisService) processJob(ctx context.Context, job *domain.MusicAnalysisJob) error {
	switch job.Kind {
	case domain.AnalysisJobKindMetadataRules:
		return service.processMetadataJob(ctx, job)
	case domain.AnalysisJobKindAudio:
		return service.processAudioJob(ctx, job)
	default:
		return newAnalyzerFailure("job_kind_invalid", false, "analysis job kind is invalid", nil)
	}
}

func (service *musicAnalysisService) processMetadataJob(ctx context.Context, job *domain.MusicAnalysisJob) error {
	if service.presetRepo == nil {
		return newAnalyzerFailure("classification_unavailable", false, "preset classification repository is unavailable", nil)
	}
	candidate, err := service.repo.FindCandidate(ctx, job.MusicID)
	if err != nil {
		return newAnalyzerFailure("music_unavailable", false, "music record is unavailable", err)
	}
	if candidate.Music.MetadataRevision != job.MetadataRevision || job.RuleVersion != domain.PresetRuleVersion {
		return errAnalysisContentStale
	}
	if _, err := service.presetRepo.Reclassify(ctx, job.MusicID); err != nil {
		return newAnalyzerFailure("classification_failed", true, "metadata classification could not be saved", err)
	}
	return nil
}

func (service *musicAnalysisService) processAudioJob(ctx context.Context, job *domain.MusicAnalysisJob) error {
	if service.analyzer == nil {
		return newAnalyzerFailure("analyzer_disabled", false, "audio analyzer is disabled", nil)
	}
	candidate, err := service.repo.FindCandidate(ctx, job.MusicID)
	if err != nil {
		return newAnalyzerFailure("music_unavailable", false, "music record is unavailable", err)
	}
	if !strings.EqualFold(candidate.FileHash, job.FileHash) {
		return errAnalysisContentStale
	}
	cached, err := service.repo.FindCachedAnalysis(ctx, job.FileHash, job.AnalyzerID, job.AnalyzerVersion, job.ModelVersion)
	if err != nil {
		return newAnalyzerFailure("analysis_cache_read_failed", true, "analysis cache could not be read", err)
	}
	if cached != nil {
		job.AnalysisID = &cached.ID
		job.ObservedFileHash = job.FileHash
		return service.applyAudioClassification(ctx, job.MusicID, cached)
	}
	if service.pathResolver == nil {
		return newAnalyzerFailure("audio_source_unavailable", false, "audio source resolver is unavailable", nil)
	}
	path, err := service.pathResolver.ResolveMusicPath(ctx, candidate.Music)
	if err != nil {
		return newAnalyzerFailure("audio_source_unavailable", IsTransientMediaStorageError(err), "audio source is temporarily unavailable", err)
	}
	file, err := os.Open(path) // #nosec G304 -- path is produced by the controlled media resolver.
	if err != nil {
		return newAnalyzerFailure("audio_source_open_failed", IsTransientMediaStorageError(err), "audio source could not be opened", err)
	}
	defer func() { _ = file.Close() }()
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Size() <= 0 {
		return newAnalyzerFailure("audio_source_invalid", false, "audio source is not a readable regular file", err)
	}
	maxBytes := int64(service.config.Analyzer.MaxFileSizeMB) * 1024 * 1024
	if info.Size() > maxBytes {
		return newAnalyzerFailure("audio_source_too_large", false, "audio source exceeded the analyzer size limit", nil)
	}

	hasher := sha256.New()
	counter := &countingWriter{}
	stream := io.TeeReader(file, io.MultiWriter(hasher, counter))
	analysisStartedAt := time.Now()
	result, analyzeErr := service.analyzer.Analyze(ctx, audioAnalyzerInput{
		MusicID: job.MusicID, FileHash: job.FileHash, ContentRevision: job.ContentRevision,
		MaxDuration:   time.Duration(service.config.Analyzer.MaxDurationSeconds) * time.Second,
		ContentLength: info.Size(), Audio: stream,
	})
	if analyzeErr != nil {
		return analyzeErr
	}
	if counter.total != info.Size() {
		return newAnalyzerFailure("analyzer_incomplete_read", true, "analyzer did not consume the complete audio stream", nil)
	}
	observedHash := hex.EncodeToString(hasher.Sum(nil))
	job.ObservedFileHash = observedHash
	if !strings.EqualFold(observedHash, job.FileHash) {
		return errAnalysisContentStale
	}
	features, err := domain.NewJSONDocument(result.Features)
	if err != nil {
		return newAnalyzerFailure("analysis_result_invalid", false, "analyzer features could not be encoded", err)
	}
	labels, err := domain.NewJSONDocument(result.ModelLabels)
	if err != nil {
		return newAnalyzerFailure("analysis_result_invalid", false, "analyzer labels could not be encoded", err)
	}
	now := time.Now().UTC()
	analysis, err := service.repo.StoreAnalysis(ctx, &domain.MusicAudioAnalysis{
		FileHash: job.FileHash, AnalyzerID: result.AnalyzerID, AnalyzerVersion: result.AnalyzerVersion,
		ModelVersion: result.ModelVersion, Status: domain.AnalysisStatusSucceeded,
		Features: features, ModelLabels: labels, DurationMS: result.DurationMS,
		ProcessingMS: time.Since(analysisStartedAt).Milliseconds(), CompletedAt: &now,
	})
	if err != nil {
		return newAnalyzerFailure("analysis_cache_write_failed", true, "analysis result could not be saved", err)
	}
	job.AnalysisID = &analysis.ID
	return service.applyAudioClassification(ctx, job.MusicID, analysis)
}

func (service *musicAnalysisService) applyAudioClassification(
	ctx context.Context,
	musicID uint,
	analysis *domain.MusicAudioAnalysis,
) error {
	if service.presetRepo == nil {
		return nil
	}
	if _, err := service.presetRepo.ReclassifyWithAudio(ctx, musicID, analysis); err != nil {
		if errors.Is(err, repository.ErrPresetAnalysisMismatch) {
			return errAnalysisContentStale
		}
		if errors.Is(err, repository.ErrMusicNotFound) {
			return newAnalyzerFailure("music_unavailable", false, "music record is unavailable", err)
		}
		return newAnalyzerFailure("classification_failed", true, "audio classification could not be saved", err)
	}
	return nil
}

func (service *musicAnalysisService) finishAttempt(job *domain.MusicAnalysisJob, attemptErr error) {
	now := time.Now().UTC()
	job.HeartbeatAt = &now
	job.FinishedAt = &now
	job.AvailableAt = nil
	job.ErrorCode = ""
	job.ErrorSummary = ""
	switch {
	case attemptErr == nil:
		job.Status = domain.AnalysisStatusSucceeded
	case errors.Is(attemptErr, errAnalysisContentStale):
		job.Status = domain.AnalysisStatusStale
		job.ErrorCode = "content_stale"
		job.ErrorSummary = "track content or metadata changed after this job was created"
	case errors.Is(attemptErr, errAnalysisCancelled):
		job.Status = service.cancelledStatus(job)
		job.ErrorCode = "cancelled"
		job.ErrorSummary = "analysis was cancelled"
	default:
		var failure *analyzerFailure
		if errors.As(attemptErr, &failure) {
			job.ErrorCode = truncateText(failure.code, 64)
			job.ErrorSummary = truncateText(failure.summary, 500)
			if failure.retryable && job.Attempt < job.MaxAttempts {
				job.Status = domain.AnalysisStatusPending
				next := now.Add(service.retryDelay(job.Attempt))
				job.AvailableAt = &next
				job.FinishedAt = nil
			} else {
				job.Status = domain.AnalysisStatusFailed
			}
		} else {
			job.Status = domain.AnalysisStatusFailed
			job.ErrorCode = "analysis_failed"
			job.ErrorSummary = "analysis failed unexpectedly"
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), analysisFinalizeTimeout)
	defer cancel()
	if err := service.repo.Complete(ctx, job); err != nil {
		if errors.Is(err, repository.ErrAnalysisJobCancelled) {
			job.Status = service.cancelledStatus(job)
			job.ErrorCode = "cancelled"
			job.ErrorSummary = "analysis was cancelled"
			job.FinishedAt = &now
			job.AvailableAt = nil
			if retryErr := service.repo.Complete(ctx, job); retryErr == nil || errors.Is(retryErr, repository.ErrAnalysisJobLeaseLost) {
				return
			}
		}
		if !errors.Is(err, repository.ErrAnalysisJobLeaseLost) {
			pklog.Errorf("Failed to finish music analysis job %d: %v", job.ID, err)
		}
	}
}

func (service *musicAnalysisService) cancelledStatus(job *domain.MusicAnalysisJob) string {
	if job.Kind == domain.AnalysisJobKindAudio {
		ctx, cancel := context.WithTimeout(context.Background(), analysisFinalizeTimeout)
		defer cancel()
		if candidate, err := service.repo.FindCandidate(ctx, job.MusicID); err == nil && !strings.EqualFold(candidate.FileHash, job.FileHash) {
			return domain.AnalysisStatusStale
		}
	}
	return domain.AnalysisStatusCancelled
}

func (service *musicAnalysisService) retryDelay(attempt int) time.Duration {
	delay := time.Duration(service.config.Analyzer.RetryInitialSeconds) * time.Second
	maximum := time.Duration(service.config.Analyzer.RetryMaxSeconds) * time.Second
	for current := 1; current < attempt && delay < maximum; current++ {
		delay *= 2
		if delay > maximum {
			delay = maximum
		}
	}
	return delay
}

func (service *musicAnalysisService) leaseDuration() time.Duration {
	duration := time.Duration(service.config.Analyzer.TimeoutSeconds)*time.Second + 30*time.Second
	if duration < time.Minute {
		return time.Minute
	}
	return duration
}

func analysisIdempotencyKey(parts ...string) string {
	hasher := sha256.New()
	for _, part := range parts {
		_, _ = hasher.Write([]byte(part))
		_, _ = hasher.Write([]byte{0})
	}
	return hex.EncodeToString(hasher.Sum(nil))
}

func validSHA256(value string) bool {
	if len(value) != sha256.Size*2 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

func newAnalysisWorkerID() string {
	buffer := make([]byte, 16)
	if _, err := rand.Read(buffer); err == nil {
		return hex.EncodeToString(buffer)
	}
	return "analysis-" + strconv.FormatInt(time.Now().UTC().UnixNano(), 10)
}

type countingWriter struct {
	total int64
}

func (writer *countingWriter) Write(value []byte) (int, error) {
	writer.total += int64(len(value))
	return len(value), nil
}
