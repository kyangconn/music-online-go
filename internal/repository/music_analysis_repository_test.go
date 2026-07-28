package repository

import (
	"context"
	"errors"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/kyangconn/music-online-go/internal/domain"
	"gorm.io/gorm"
)

func TestMusicAnalysisRepositoryQueueIsIdempotentAndBounded(t *testing.T) {
	db := openMusicAnalysisRepositoryTestDB(t)
	repo := NewMusicAnalysisRepository(db)
	ctx := context.Background()
	music := createAnalysisMusic(t, db, strings.Repeat("a", 64))

	job := newRepositoryAnalysisJob(music.ID, "job-one", music.FileHash, 2)
	queued, created, err := repo.Enqueue(ctx, job, false, 1)
	if err != nil || !created || queued.ID == 0 {
		t.Fatalf("enqueue first job: job=%+v created=%v err=%v", queued, created, err)
	}
	reused, created, err := repo.Enqueue(ctx, newRepositoryAnalysisJob(music.ID, "job-one", music.FileHash, 2), false, 1)
	if err != nil || created || reused.ID != queued.ID {
		t.Fatalf("repeat enqueue was not idempotent: job=%+v created=%v err=%v", reused, created, err)
	}
	if _, _, err := repo.Enqueue(ctx, newRepositoryAnalysisJob(music.ID, "job-one", music.FileHash, 2), true, 1); !errors.Is(err, ErrAnalysisJobActive) {
		t.Fatalf("forcing an active job error = %v, want ErrAnalysisJobActive", err)
	}

	other := createAnalysisMusic(t, db, strings.Repeat("b", 64))
	if _, _, err := repo.Enqueue(ctx, newRepositoryAnalysisJob(other.ID, "job-two", other.FileHash, 2), false, 1); !errors.Is(err, ErrAnalysisQueueFull) {
		t.Fatalf("queue limit error = %v, want ErrAnalysisQueueFull", err)
	}

	claimed, found, err := repo.ClaimNext(ctx, "worker-a", time.Minute)
	if err != nil || !found || claimed.ID != queued.ID || claimed.Attempt != 1 || claimed.LeaseGeneration != 1 {
		t.Fatalf("claim job: job=%+v found=%v err=%v", claimed, found, err)
	}
	now := time.Now().UTC()
	claimed.Status = domain.AnalysisStatusSucceeded
	claimed.FinishedAt = &now
	if err := repo.Complete(ctx, claimed); err != nil {
		t.Fatalf("complete job: %v", err)
	}
	reset, changed, err := repo.Enqueue(ctx, newRepositoryAnalysisJob(music.ID, "job-one", music.FileHash, 2), true, 1)
	if err != nil || !changed || reset.ID != queued.ID || reset.Status != domain.AnalysisStatusPending || reset.Attempt != 0 {
		t.Fatalf("force reset terminal job: job=%+v changed=%v err=%v", reset, changed, err)
	}
}

func TestMusicAnalysisRepositoryRecoversLeasesAndCancellation(t *testing.T) {
	db := openMusicAnalysisRepositoryTestDB(t)
	repo := NewMusicAnalysisRepository(db)
	ctx := context.Background()
	music := createAnalysisMusic(t, db, strings.Repeat("c", 64))

	for index, maxAttempts := range []int{2, 1, 2} {
		job := newRepositoryAnalysisJob(music.ID, "recover-"+string(rune('a'+index)), music.FileHash, maxAttempts)
		if _, _, err := repo.Enqueue(ctx, job, false, 10); err != nil {
			t.Fatalf("enqueue recovery job: %v", err)
		}
		claimed, found, err := repo.ClaimNext(ctx, "worker-"+string(rune('a'+index)), time.Minute)
		if err != nil || !found {
			t.Fatalf("claim recovery job: found=%v err=%v", found, err)
		}
		expired := time.Now().UTC().Add(-time.Minute)
		updates := map[string]any{"lease_expires_at": expired}
		if index == 2 {
			updates["cancel_requested"] = true
		}
		if err := db.Model(&domain.MusicAnalysisJob{}).Where("id = ?", claimed.ID).Updates(updates).Error; err != nil {
			t.Fatalf("expire job lease: %v", err)
		}
	}
	if err := repo.RecoverExpired(ctx); err != nil {
		t.Fatalf("recover expired jobs: %v", err)
	}
	var jobs []*domain.MusicAnalysisJob
	if err := db.Order("id ASC").Find(&jobs).Error; err != nil {
		t.Fatalf("list recovered jobs: %v", err)
	}
	if jobs[0].Status != domain.AnalysisStatusPending || jobs[0].ErrorCode != "worker_lease_expired" {
		t.Fatalf("retryable expired job = %+v", jobs[0])
	}
	if jobs[1].Status != domain.AnalysisStatusFailed || jobs[1].FinishedAt == nil {
		t.Fatalf("exhausted expired job = %+v", jobs[1])
	}
	if jobs[2].Status != domain.AnalysisStatusCancelled || jobs[2].FinishedAt == nil {
		t.Fatalf("cancelled expired job = %+v", jobs[2])
	}
}

func TestMusicAnalysisRepositoryCancellationWinsOverRetryCompletion(t *testing.T) {
	db := openMusicAnalysisRepositoryTestDB(t)
	repo := NewMusicAnalysisRepository(db)
	ctx := context.Background()
	music := createAnalysisMusic(t, db, strings.Repeat("7", 64))

	if _, _, err := repo.Enqueue(ctx, newRepositoryAnalysisJob(music.ID, "cancel-retry", music.FileHash, 2), false, 10); err != nil {
		t.Fatalf("enqueue cancellation fixture: %v", err)
	}
	claimed, found, err := repo.ClaimNext(ctx, "worker", time.Minute)
	if err != nil || !found {
		t.Fatalf("claim cancellation fixture: found=%v err=%v", found, err)
	}
	if err := db.Model(&domain.MusicAnalysisJob{}).Where("id = ?", claimed.ID).Update("cancel_requested", true).Error; err != nil {
		t.Fatalf("flag cancellation: %v", err)
	}
	next := time.Now().UTC().Add(time.Minute)
	claimed.Status = domain.AnalysisStatusPending
	claimed.AvailableAt = &next
	if err := repo.Complete(ctx, claimed); !errors.Is(err, ErrAnalysisJobCancelled) {
		t.Fatalf("retry completion error = %v, want ErrAnalysisJobCancelled", err)
	}

	finished := time.Now().UTC()
	claimed.Status = domain.AnalysisStatusCancelled
	claimed.ErrorCode = "cancelled"
	claimed.AvailableAt = nil
	claimed.FinishedAt = &finished
	if err := repo.Complete(ctx, claimed); err != nil {
		t.Fatalf("finalize cancelled job: %v", err)
	}
	stored, err := repo.FindJob(ctx, claimed.ID)
	if err != nil || stored.Status != domain.AnalysisStatusCancelled || !stored.CancelRequested {
		t.Fatalf("stored cancellation = %+v, err=%v", stored, err)
	}
}

func TestMusicAnalysisRepositorySkipsFlaggedPendingJob(t *testing.T) {
	db := openMusicAnalysisRepositoryTestDB(t)
	repo := NewMusicAnalysisRepository(db)
	ctx := context.Background()
	music := createAnalysisMusic(t, db, strings.Repeat("8", 64))

	queued, _, err := repo.Enqueue(ctx, newRepositoryAnalysisJob(music.ID, "flagged-pending", music.FileHash, 2), false, 10)
	if err != nil {
		t.Fatalf("enqueue pending fixture: %v", err)
	}
	if err := db.Model(&domain.MusicAnalysisJob{}).Where("id = ?", queued.ID).Update("cancel_requested", true).Error; err != nil {
		t.Fatalf("flag pending fixture: %v", err)
	}
	if claimed, found, err := repo.ClaimNext(ctx, "worker", time.Minute); err != nil || found || claimed != nil {
		t.Fatalf("flagged pending job was claimable: job=%+v found=%v err=%v", claimed, found, err)
	}
}

func TestMusicAnalysisRepositoryReturnsOnlyLatestAudioJob(t *testing.T) {
	db := openMusicAnalysisRepositoryTestDB(t)
	repo := NewMusicAnalysisRepository(db)
	ctx := context.Background()
	first := createAnalysisMusic(t, db, strings.Repeat("9", 64))
	second := createAnalysisMusic(t, db, strings.Repeat("a", 64))

	var latestFirst uint
	for index, musicID := range []uint{first.ID, first.ID, second.ID} {
		job := newRepositoryAnalysisJob(musicID, "latest-"+strconv.Itoa(index), strings.Repeat("9", 64), 1)
		job.Status = domain.AnalysisStatusSucceeded
		job.ProcessingMS = int64(index + 1)
		if err := db.Create(job).Error; err != nil {
			t.Fatalf("create historical job: %v", err)
		}
		if musicID == first.ID {
			latestFirst = job.ID
		}
	}
	latest, err := repo.LatestAudioJobsByMusicIDs(ctx, []uint{first.ID, second.ID})
	if err != nil {
		t.Fatalf("load latest jobs: %v", err)
	}
	if len(latest) != 2 || latest[first.ID] == nil || latest[first.ID].ID != latestFirst || latest[second.ID] == nil {
		t.Fatalf("latest jobs = %+v", latest)
	}
	metrics, err := repo.Metrics(ctx)
	if err != nil || metrics.AverageProcessing != 2 {
		t.Fatalf("queue metrics = %+v, err=%v", metrics, err)
	}
}

func TestMusicAnalysisRepositorySharesArtifactsAndDeletesOnlyOrphans(t *testing.T) {
	db := openMusicAnalysisRepositoryTestDB(t)
	repo := NewMusicAnalysisRepository(db)
	ctx := context.Background()
	hash := strings.Repeat("d", 64)
	first := createAnalysisMusic(t, db, hash)
	second := createAnalysisMusic(t, db, hash)
	now := time.Now().UTC()
	document, err := domain.NewJSONDocument(map[string]float64{"bpm": 128})
	if err != nil {
		t.Fatal(err)
	}
	analysis, err := repo.StoreAnalysis(ctx, &domain.MusicAudioAnalysis{
		FileHash: hash, AnalyzerID: "fixture", AnalyzerVersion: "1", ModelVersion: "m1",
		Status: domain.AnalysisStatusSucceeded, Features: document, ModelLabels: document, CompletedAt: &now,
	})
	if err != nil {
		t.Fatalf("store analysis: %v", err)
	}
	duplicate, err := repo.StoreAnalysis(ctx, &domain.MusicAudioAnalysis{
		FileHash: hash, AnalyzerID: "fixture", AnalyzerVersion: "1", ModelVersion: "m1",
		Status: domain.AnalysisStatusSucceeded, Features: document, ModelLabels: document, CompletedAt: &now,
	})
	if err != nil || duplicate.ID != analysis.ID {
		t.Fatalf("reuse analysis artifact: duplicate=%+v err=%v", duplicate, err)
	}
	for index, musicID := range []uint{first.ID, second.ID} {
		job := newRepositoryAnalysisJob(musicID, "shared-"+string(rune('a'+index)), hash, 1)
		job.Status = domain.AnalysisStatusSucceeded
		job.AnalysisID = &analysis.ID
		job.FinishedAt = &now
		if err := db.Create(job).Error; err != nil {
			t.Fatalf("create completed shared job: %v", err)
		}
	}
	if err := db.Transaction(func(tx *gorm.DB) error { return deleteMusicAnalysisState(tx, []uint{first.ID}) }); err != nil {
		t.Fatalf("delete first analysis state: %v", err)
	}
	var count int64
	if err := db.Model(&domain.MusicAudioAnalysis{}).Count(&count).Error; err != nil || count != 1 {
		t.Fatalf("shared artifact count = %d, err=%v", count, err)
	}
	if err := db.Transaction(func(tx *gorm.DB) error { return deleteMusicAnalysisState(tx, []uint{second.ID}) }); err != nil {
		t.Fatalf("delete second analysis state: %v", err)
	}
	if err := db.Model(&domain.MusicAudioAnalysis{}).Count(&count).Error; err != nil || count != 0 {
		t.Fatalf("orphan artifact count = %d, err=%v", count, err)
	}
}

func TestMusicAnalysisRepositoryMarksSupersededJobsStale(t *testing.T) {
	db := openMusicAnalysisRepositoryTestDB(t)
	repo := NewMusicAnalysisRepository(db)
	ctx := context.Background()
	music := createAnalysisMusic(t, db, strings.Repeat("e", 64))
	oldHash := strings.Repeat("f", 64)
	pending := newRepositoryAnalysisJob(music.ID, "old-pending", oldHash, 2)
	running := newRepositoryAnalysisJob(music.ID, "old-running", oldHash, 2)
	if _, _, err := repo.Enqueue(ctx, pending, false, 10); err != nil {
		t.Fatal(err)
	}
	if _, _, err := repo.Enqueue(ctx, running, false, 10); err != nil {
		t.Fatal(err)
	}
	if _, found, err := repo.ClaimNext(ctx, "worker", time.Minute); err != nil || !found {
		t.Fatalf("claim old job: found=%v err=%v", found, err)
	}
	if err := repo.MarkSupersededAudio(ctx, music.ID, music.FileHash); err != nil {
		t.Fatalf("mark superseded: %v", err)
	}
	var staleCount int64
	if err := db.Model(&domain.MusicAnalysisJob{}).
		Where("status = ? AND error_code = ?", domain.AnalysisStatusStale, "content_superseded").Count(&staleCount).Error; err != nil || staleCount != 1 {
		t.Fatalf("stale superseded count = %d, err=%v", staleCount, err)
	}
	var runningCount int64
	if err := db.Model(&domain.MusicAnalysisJob{}).Where("status = ? AND cancel_requested = ?", domain.AnalysisStatusRunning, true).Count(&runningCount).Error; err != nil || runningCount != 1 {
		t.Fatalf("running superseded cancellation count = %d, err=%v", runningCount, err)
	}
}

func openMusicAnalysisRepositoryTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "analysis.db")), &gorm.Config{})
	if err != nil {
		t.Fatalf("open analysis database: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get analysis database handle: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() {
		if err := sqlDB.Close(); err != nil {
			t.Errorf("close analysis database: %v", err)
		}
	})
	if err := db.AutoMigrate(&domain.User{}, &domain.Music{}, &domain.MediaFile{}, &domain.MusicAudioAnalysis{}, &domain.MusicAnalysisJob{}); err != nil {
		t.Fatalf("migrate analysis database: %v", err)
	}
	return db
}

func createAnalysisMusic(t *testing.T, db *gorm.DB, fileHash string) *domain.Music {
	t.Helper()
	music := &domain.Music{Title: "Track", Artist: "Artist", FileHash: fileHash, MetadataRevision: 1}
	if err := db.Create(music).Error; err != nil {
		t.Fatalf("create analysis music: %v", err)
	}
	mediaFile := &domain.MediaFile{
		MusicID: music.ID, RootID: domain.ManagedMediaRootID, RelativePath: "track.flac",
		SourceKey: "source-" + strconv.FormatUint(uint64(music.ID), 10), FileHash: fileHash, ObservedFileHash: fileHash,
		Availability: domain.MediaFileAvailabilityOnline, ContentRevision: 1,
	}
	if err := db.Create(mediaFile).Error; err != nil {
		t.Fatalf("create analysis media file: %v", err)
	}
	return music
}

func newRepositoryAnalysisJob(musicID uint, key, fileHash string, attempts int) *domain.MusicAnalysisJob {
	return &domain.MusicAnalysisJob{
		Kind: domain.AnalysisJobKindAudio, IdempotencyKey: key, MusicID: musicID,
		FileHash: fileHash, AnalyzerID: "fixture", AnalyzerVersion: "1", ModelVersion: "m1",
		Status: domain.AnalysisStatusPending, MaxAttempts: attempts,
	}
}
