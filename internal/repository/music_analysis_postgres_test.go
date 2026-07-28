package repository

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/kyangconn/music-online-go/internal/domain"
)

// TestMusicAnalysisRepositoryPostgres is opt-in because ordinary development
// and CI do not require a PostgreSQL daemon. The supplied database may contain
// other data: the test creates and drops only its own randomly named schema.
func TestMusicAnalysisRepositoryPostgres(t *testing.T) {
	db := openMusicAnalysisPostgresTestDB(t)
	repo := NewMusicAnalysisRepository(db)
	ctx := context.Background()
	document, err := domain.NewJSONDocument(map[string]float64{"bpm": 128})
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	artifact, err := repo.StoreAnalysis(ctx, &domain.MusicAudioAnalysis{
		FileHash: strings.Repeat("1", 64), AnalyzerID: "postgres-fixture", AnalyzerVersion: "1",
		ModelVersion: "m1", Status: domain.AnalysisStatusSucceeded, Features: document,
		ModelLabels: document, DurationMS: 1000, ProcessingMS: 12, CompletedAt: &now,
	})
	if err != nil {
		t.Fatalf("store PostgreSQL artifact: %v", err)
	}

	first := createAnalysisMusic(t, db, strings.Repeat("1", 64))
	second := createAnalysisMusic(t, db, strings.Repeat("2", 64))
	for index, music := range []*domain.Music{first, second} {
		job := newRepositoryAnalysisJob(music.ID, fmt.Sprintf("postgres-job-%d", index), music.FileHash, 2)
		if _, created, err := repo.Enqueue(ctx, job, false, 10); err != nil || !created {
			t.Fatalf("enqueue PostgreSQL job %d: created=%v err=%v", index, created, err)
		}
	}

	type claimResult struct {
		job   *domain.MusicAnalysisJob
		found bool
		err   error
	}
	claims := make(chan claimResult, 2)
	var workers sync.WaitGroup
	for index := 0; index < 2; index++ {
		workers.Add(1)
		go func(index int) {
			defer workers.Done()
			job, found, claimErr := repo.ClaimNext(ctx, fmt.Sprintf("postgres-worker-%d", index), time.Minute)
			claims <- claimResult{job: job, found: found, err: claimErr}
		}(index)
	}
	workers.Wait()
	close(claims)
	claimed := make(map[uint]*domain.MusicAnalysisJob, 2)
	for result := range claims {
		if result.err != nil || !result.found || result.job == nil {
			t.Fatalf("claim PostgreSQL job: found=%v job=%+v err=%v", result.found, result.job, result.err)
		}
		claimed[result.job.MusicID] = result.job
	}
	if len(claimed) != 2 {
		t.Fatalf("concurrent PostgreSQL claims were not distinct: %+v", claimed)
	}

	succeeded := claimed[first.ID]
	succeeded.Status = domain.AnalysisStatusSucceeded
	succeeded.AnalysisID = &artifact.ID
	succeeded.ProcessingMS = 12
	succeeded.FinishedAt = &now
	if err := repo.Complete(ctx, succeeded); err != nil {
		t.Fatalf("complete PostgreSQL job: %v", err)
	}
	cancelled := claimed[second.ID]
	if _, err := repo.RequestCancellation(ctx, cancelled.ID); err != nil {
		t.Fatalf("request PostgreSQL cancellation: %v", err)
	}
	cancelled.Status = domain.AnalysisStatusCancelled
	cancelled.ErrorCode = "cancelled"
	cancelled.FinishedAt = &now
	if err := repo.Complete(ctx, cancelled); err != nil {
		t.Fatalf("complete PostgreSQL cancellation: %v", err)
	}

	latest, err := repo.LatestAudioJobsByMusicIDs(ctx, []uint{first.ID, second.ID})
	if err != nil || len(latest) != 2 || latest[first.ID] == nil || latest[second.ID] == nil {
		t.Fatalf("latest PostgreSQL jobs = %+v, err=%v", latest, err)
	}
	metrics, err := repo.Metrics(ctx)
	if err != nil || metrics.Statuses[domain.AnalysisStatusSucceeded] != 1 || metrics.Statuses[domain.AnalysisStatusCancelled] != 1 {
		t.Fatalf("PostgreSQL metrics = %+v, err=%v", metrics, err)
	}
	loaded, err := repo.FindJob(ctx, succeeded.ID)
	if err != nil || loaded.Analysis == nil || string(loaded.Analysis.Features) != string(document) {
		t.Fatalf("load PostgreSQL JSON artifact: job=%+v err=%v", loaded, err)
	}
	if err := db.Transaction(func(tx *gorm.DB) error { return deleteMusicAnalysisState(tx, []uint{first.ID}) }); err != nil {
		t.Fatalf("delete PostgreSQL analysis state: %v", err)
	}
	if cached, err := repo.FindCachedAnalysis(ctx, artifact.FileHash, artifact.AnalyzerID, artifact.AnalyzerVersion, artifact.ModelVersion); err != nil || cached != nil {
		t.Fatalf("orphan PostgreSQL artifact was not removed: artifact=%+v err=%v", cached, err)
	}
}

func openMusicAnalysisPostgresTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := strings.TrimSpace(os.Getenv("MUSIC_ONLINE_TEST_POSTGRES_DSN"))
	if dsn == "" {
		t.Skip("set MUSIC_ONLINE_TEST_POSTGRES_DSN to run PostgreSQL integration tests")
	}
	parsed, err := url.Parse(dsn)
	if err != nil || (parsed.Scheme != "postgres" && parsed.Scheme != "postgresql") || parsed.Host == "" {
		t.Fatalf("MUSIC_ONLINE_TEST_POSTGRES_DSN must be a PostgreSQL URL")
	}
	gormConfig := &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)}
	admin, err := gorm.Open(postgres.Open(parsed.String()), gormConfig)
	if err != nil {
		t.Fatalf("connect PostgreSQL integration database: %v", err)
	}
	adminSQL, err := admin.DB()
	if err != nil {
		t.Fatalf("get PostgreSQL integration database handle: %v", err)
	}
	schema := fmt.Sprintf("music_online_test_%d", time.Now().UTC().UnixNano())
	if err := admin.Exec(`CREATE SCHEMA "` + schema + `"`).Error; err != nil {
		_ = adminSQL.Close()
		t.Fatalf("create PostgreSQL integration schema: %v", err)
	}

	query := parsed.Query()
	query.Set("search_path", schema)
	parsed.RawQuery = query.Encode()
	db, err := gorm.Open(postgres.Open(parsed.String()), gormConfig)
	if err != nil {
		_ = admin.Exec(`DROP SCHEMA "` + schema + `" CASCADE`).Error
		_ = adminSQL.Close()
		t.Fatalf("open isolated PostgreSQL integration schema: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get isolated PostgreSQL database handle: %v", err)
	}
	sqlDB.SetMaxOpenConns(4)
	t.Cleanup(func() {
		_ = sqlDB.Close()
		if err := admin.Exec(`DROP SCHEMA "` + schema + `" CASCADE`).Error; err != nil {
			t.Errorf("drop PostgreSQL integration schema: %v", err)
		}
		_ = adminSQL.Close()
	})
	if err := db.AutoMigrate(
		&domain.User{}, &domain.Music{}, &domain.MediaFile{},
		&domain.MusicAudioAnalysis{}, &domain.MusicAnalysisJob{},
	); err != nil {
		t.Fatalf("migrate PostgreSQL analysis schema: %v", err)
	}
	for _, index := range []string{"idx_analysis_claim", "idx_analysis_music_kind_id"} {
		if !db.Migrator().HasIndex(&domain.MusicAnalysisJob{}, index) {
			t.Fatalf("PostgreSQL analysis index %q is missing", index)
		}
	}
	return db
}
