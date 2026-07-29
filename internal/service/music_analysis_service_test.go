package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/kyangconn/music-online-go/internal/config"
	"github.com/kyangconn/music-online-go/internal/domain"
	"github.com/kyangconn/music-online-go/internal/repository"
	"gorm.io/gorm"
)

func TestMusicAnalysisWorkerReusesContentArtifact(t *testing.T) {
	db := openAnalysisServiceTestDB(t)
	audioPath, fileHash := writeAnalysisFixture(t, []byte("shared analyzer fixture"))
	first := createAnalysisServiceMusic(t, db, fileHash, "first")
	second := createAnalysisServiceMusic(t, db, fileHash, "second")
	resolver := &analysisPathResolver{paths: map[uint]string{first.ID: audioPath, second.ID: audioPath}}

	var requests atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		requests.Add(1)
		body, err := io.ReadAll(request.Body)
		if err != nil || string(body) != "shared analyzer fixture" {
			t.Errorf("analyzer stream = %q, err=%v", body, err)
		}
		time.Sleep(10 * time.Millisecond)
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"analyzer_id":"fixture","analyzer_version":"1.0","model_version":"m1","duration_ms":1000,"features":{"bpm":128},"model_labels":{"trance":0.8}}`))
	}))
	defer server.Close()

	repo := repository.NewMusicAnalysisRepository(db)
	presetRepo := repository.NewPresetRepository(db, domain.DefaultPresetRulePolicy())
	cfg := analysisServiceConfig(server.URL)
	service := NewMusicAnalysisService(repo, presetRepo, resolver, cfg)
	firstSchedule, err := service.ScheduleMusic(context.Background(), first.ID, 1, domain.AnalysisEnqueueRequest{IncludeAudio: true})
	if err != nil {
		t.Fatalf("schedule first music: %v", err)
	}
	secondSchedule, err := service.ScheduleMusic(context.Background(), second.ID, 1, domain.AnalysisEnqueueRequest{IncludeAudio: true})
	if err != nil {
		t.Fatalf("schedule second music: %v", err)
	}
	startAnalysisService(t, service)
	waitForAnalysisJobStatus(t, repo, firstSchedule.AudioJob.ID, domain.AnalysisStatusSucceeded, 6*time.Second)
	waitForAnalysisJobStatus(t, repo, secondSchedule.AudioJob.ID, domain.AnalysisStatusSucceeded, 6*time.Second)
	if requests.Load() != 1 {
		t.Fatalf("analyzer requests = %d, want one shared-content request", requests.Load())
	}
	firstJob, _ := repo.FindJob(context.Background(), firstSchedule.AudioJob.ID)
	secondJob, _ := repo.FindJob(context.Background(), secondSchedule.AudioJob.ID)
	if firstJob.AnalysisID == nil || secondJob.AnalysisID == nil || *firstJob.AnalysisID != *secondJob.AnalysisID {
		t.Fatalf("analysis artifacts were not shared: first=%+v second=%+v", firstJob.AnalysisID, secondJob.AnalysisID)
	}
	if firstJob.Analysis == nil || firstJob.Analysis.ProcessingMS <= 0 {
		t.Fatalf("analysis processing time was not persisted: %+v", firstJob.Analysis)
	}
	classifications, err := presetRepo.FindByMusicIDs(context.Background(), []uint{first.ID, second.ID})
	if err != nil {
		t.Fatal(err)
	}
	for _, music := range []*domain.Music{first, second} {
		classification := classifications[music.ID]
		if classification == nil || classification.RuleVersion != domain.PresetHybridRuleVersion ||
			classification.AudioAnalysisID == nil || *classification.AudioAnalysisID != *firstJob.AnalysisID {
			t.Fatalf("music %d hybrid classification = %+v", music.ID, classification)
		}
	}
}

func TestMusicAnalysisWorkerCancelsRunningAnalyzer(t *testing.T) {
	db := openAnalysisServiceTestDB(t)
	audioPath, fileHash := writeAnalysisFixture(t, []byte("blocking fixture"))
	music := createAnalysisServiceMusic(t, db, fileHash, "cancel")
	repo := repository.NewMusicAnalysisRepository(db)
	presetRepo := repository.NewPresetRepository(db, domain.DefaultPresetRulePolicy())
	analyzer := &blockingAudioAnalyzer{started: make(chan struct{})}
	cfg := analysisServiceConfig("http://unused.invalid")
	cfg.Analyzer.RetryMaxAttempts = 1
	service := newMusicAnalysisServiceWithAnalyzer(repo, presetRepo, &analysisPathResolver{paths: map[uint]string{music.ID: audioPath}}, cfg, analyzer)
	schedule, err := service.ScheduleMusic(context.Background(), music.ID, 1, domain.AnalysisEnqueueRequest{IncludeAudio: true})
	if err != nil {
		t.Fatalf("schedule cancellation fixture: %v", err)
	}
	startAnalysisService(t, service)
	select {
	case <-analyzer.started:
	case <-time.After(4 * time.Second):
		t.Fatal("analyzer did not start")
	}
	if _, err := service.CancelJob(context.Background(), schedule.AudioJob.ID); err != nil {
		t.Fatalf("request analysis cancellation: %v", err)
	}
	waitForAnalysisJobStatus(t, repo, schedule.AudioJob.ID, domain.AnalysisStatusCancelled, 8*time.Second)
}

func TestMusicAnalysisWorkerRetriesTransientFailure(t *testing.T) {
	db := openAnalysisServiceTestDB(t)
	audioPath, fileHash := writeAnalysisFixture(t, []byte("retry fixture"))
	music := createAnalysisServiceMusic(t, db, fileHash, "retry")
	repo := repository.NewMusicAnalysisRepository(db)
	presetRepo := repository.NewPresetRepository(db, domain.DefaultPresetRulePolicy())
	analyzer := &retryAudioAnalyzer{}
	cfg := analysisServiceConfig("http://unused.invalid")
	cfg.Analyzer.RetryInitialSeconds = 1
	cfg.Analyzer.RetryMaxSeconds = 1
	service := newMusicAnalysisServiceWithAnalyzer(repo, presetRepo, &analysisPathResolver{paths: map[uint]string{music.ID: audioPath}}, cfg, analyzer)
	schedule, err := service.ScheduleMusic(context.Background(), music.ID, 1, domain.AnalysisEnqueueRequest{IncludeAudio: true})
	if err != nil {
		t.Fatalf("schedule retry fixture: %v", err)
	}
	startAnalysisService(t, service)
	job := waitForAnalysisJobStatus(t, repo, schedule.AudioJob.ID, domain.AnalysisStatusSucceeded, 8*time.Second)
	if job.Attempt != 2 || analyzer.calls.Load() != 2 {
		t.Fatalf("retry job attempt=%d analyzer calls=%d", job.Attempt, analyzer.calls.Load())
	}
}

func TestMusicAnalysisWorkerMarksChangedContentStaleWithoutCallingAnalyzer(t *testing.T) {
	db := openAnalysisServiceTestDB(t)
	audioPath, fileHash := writeAnalysisFixture(t, []byte("stale fixture"))
	music := createAnalysisServiceMusic(t, db, fileHash, "stale")
	repo := repository.NewMusicAnalysisRepository(db)
	presetRepo := repository.NewPresetRepository(db, domain.DefaultPresetRulePolicy())
	analyzer := &retryAudioAnalyzer{}
	cfg := analysisServiceConfig("http://unused.invalid")
	service := newMusicAnalysisServiceWithAnalyzer(repo, presetRepo, &analysisPathResolver{paths: map[uint]string{music.ID: audioPath}}, cfg, analyzer)
	schedule, err := service.ScheduleMusic(context.Background(), music.ID, 1, domain.AnalysisEnqueueRequest{IncludeAudio: true})
	if err != nil {
		t.Fatalf("schedule stale fixture: %v", err)
	}
	newHash := strings.Repeat("9", 64)
	if err := db.Model(&domain.Music{}).Where("id = ?", music.ID).Update("file_hash", newHash).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&domain.MediaFile{}).Where("music_id = ?", music.ID).Updates(map[string]any{"file_hash": newHash, "observed_file_hash": newHash, "content_revision": 2}).Error; err != nil {
		t.Fatal(err)
	}
	startAnalysisService(t, service)
	waitForAnalysisJobStatus(t, repo, schedule.AudioJob.ID, domain.AnalysisStatusStale, 6*time.Second)
	if analyzer.calls.Load() != 0 {
		t.Fatalf("stale job called analyzer %d time(s)", analyzer.calls.Load())
	}
}

type analysisPathResolver struct {
	paths map[uint]string
}

func (resolver *analysisPathResolver) ResolveMusicPath(_ context.Context, music *domain.Music) (string, error) {
	path := resolver.paths[music.ID]
	if path == "" {
		return "", ErrMediaNotFound
	}
	return path, nil
}

func (*analysisPathResolver) HasReadOnlyMediaSource(context.Context, uint) (bool, error) {
	return false, nil
}

type blockingAudioAnalyzer struct {
	started chan struct{}
	once    sync.Once
}

func (analyzer *blockingAudioAnalyzer) Analyze(ctx context.Context, input audioAnalyzerInput) (*audioAnalyzerResult, error) {
	analyzer.once.Do(func() { close(analyzer.started) })
	_, _ = io.Copy(io.Discard, input.Audio)
	<-ctx.Done()
	return nil, newAnalyzerFailure("analyzer_cancelled", false, "analyzer request was cancelled", ctx.Err())
}

type retryAudioAnalyzer struct {
	calls atomic.Int64
}

func (analyzer *retryAudioAnalyzer) Analyze(_ context.Context, input audioAnalyzerInput) (*audioAnalyzerResult, error) {
	call := analyzer.calls.Add(1)
	_, _ = io.Copy(io.Discard, input.Audio)
	if call == 1 {
		return nil, newAnalyzerFailure("analyzer_unavailable", true, "analyzer could not be reached", errors.New("fixture unavailable"))
	}
	return &audioAnalyzerResult{
		AnalyzerID: "fixture", AnalyzerVersion: "1.0", ModelVersion: "m1", DurationMS: 1000,
		Features: map[string]any{"bpm": float64(128)}, ModelLabels: map[string]float64{"trance": 0.8},
	}, nil
}

func openAnalysisServiceTestDB(t *testing.T) *gorm.DB {
	return openMediaLibraryTestDB(t, "music-analysis.db",
		&domain.User{}, &domain.Music{}, &domain.MediaFile{},
		&domain.MusicPresetClassification{}, &domain.MusicPresetScore{},
		&domain.MusicAudioAnalysis{}, &domain.MusicAnalysisJob{},
	)
}

func writeAnalysisFixture(t *testing.T, content []byte) (string, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "fixture.flac")
	if err := os.WriteFile(path, content, 0600); err != nil {
		t.Fatalf("write analysis fixture: %v", err)
	}
	hash := sha256.Sum256(content)
	return path, hex.EncodeToString(hash[:])
}

func createAnalysisServiceMusic(t *testing.T, db *gorm.DB, fileHash, sourceKey string) *domain.Music {
	t.Helper()
	music := &domain.Music{Title: "Track", Artist: "Artist", Genres: domain.StringList{"Trance"}, FileHash: fileHash, MetadataRevision: 1}
	if err := db.Create(music).Error; err != nil {
		t.Fatalf("create analysis service music: %v", err)
	}
	mediaFile := &domain.MediaFile{
		MusicID: music.ID, RootID: domain.ManagedMediaRootID, RelativePath: sourceKey + ".flac", SourceKey: sourceKey,
		FileHash: fileHash, ObservedFileHash: fileHash, Availability: domain.MediaFileAvailabilityOnline, ContentRevision: 1,
	}
	if err := db.Create(mediaFile).Error; err != nil {
		t.Fatalf("create analysis service media file: %v", err)
	}
	return music
}

func analysisServiceConfig(endpoint string) config.ClassificationConfig {
	return config.ClassificationConfig{
		Enabled: true, AutoThreshold: 0.65, ReviewMargin: 0.12,
		CalmFlowWeight: 1, KineticPulseWeight: 1, CosmicDriftWeight: 1, BassImpactWeight: 1,
		Analyzer: config.AnalyzerConfig{
			Mode: "http", Endpoint: endpoint, Token: strings.Repeat("t", 32), ID: "fixture", Version: "1.0", ModelVersion: "m1",
			TimeoutSeconds: 5, Concurrency: 1, QueueLimit: 100, MaxFileSizeMB: 1, MaxDurationSeconds: 60,
			RetryMaxAttempts: 2, RetryInitialSeconds: 1, RetryMaxSeconds: 1,
		},
	}
}

func startAnalysisService(t *testing.T, service MusicAnalysisService) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	if err := service.Start(ctx); err != nil {
		cancel()
		t.Fatalf("start analysis service: %v", err)
	}
	t.Cleanup(func() {
		cancel()
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer shutdownCancel()
		if err := service.Shutdown(shutdownCtx); err != nil {
			t.Errorf("shutdown analysis service: %v", err)
		}
	})
}

func waitForAnalysisJobStatus(t *testing.T, repo repository.MusicAnalysisRepository, id uint, status string, timeout time.Duration) *domain.MusicAnalysisJob {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		job, err := repo.FindJob(context.Background(), id)
		if err == nil && job.Status == status {
			return job
		}
		time.Sleep(25 * time.Millisecond)
	}
	job, err := repo.FindJob(context.Background(), id)
	t.Fatalf("job %d did not reach %q: job=%+v err=%v", id, status, job, err)
	return nil
}
