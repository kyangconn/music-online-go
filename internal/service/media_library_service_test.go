package service

import (
	"context"
	"encoding/binary"
	"errors"
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/kyangconn/music-online-go/internal/config"
	"github.com/kyangconn/music-online-go/internal/domain"
	"github.com/kyangconn/music-online-go/internal/mediafs"
	"github.com/kyangconn/music-online-go/internal/repository"
	"gorm.io/gorm"
)

func TestMediaLibraryScanIsAdditiveAndPreservesChangedSources(t *testing.T) {
	db := openMediaLibraryTestDB(t, "library.db",
		&domain.Music{},
		&domain.MediaLibraryRoot{},
		&domain.MediaLibraryRootState{},
		&domain.MediaFile{},
		&domain.MediaScanJob{},
		&domain.MediaScanIssue{},
	)

	managedDir := t.TempDir()
	externalDir := t.TempDir()
	originalConfig := config.AppConfig
	config.AppConfig = &config.Config{Server: config.ServerConfig{UploadDir: managedDir, MaxCoverSizeMB: 1}}
	t.Cleanup(func() { config.AppConfig = originalConfig })

	libraryRepo := repository.NewMediaLibraryRepository(db)
	musicRepo := repository.NewMusicRepository(db)
	root := &domain.MediaLibraryRoot{Name: "Archive", Path: externalDir, Enabled: true, ReadOnly: true, CreatedBy: 1}
	if err := libraryRepo.CreateRoot(context.Background(), root); err != nil {
		t.Fatalf("create root: %v", err)
	}
	path := filepath.Join(externalDir, "track.wav")
	writeMinimalWAV(t, path, "Track", "Artist", 8000)

	svc := NewMediaLibraryService(libraryRepo, musicRepo, config.LibraryConfig{
		Scanner: config.LibraryScannerConfig{
			Enabled: true, MaxFileSizeMB: 1, MaxTagSizeMB: 16, MinFileAgeSeconds: 0,
			RetryMaxAttempts: 3, RetryInitialSeconds: 1, RetryMaxSeconds: 2,
		},
	}).(*mediaLibraryService)
	first := runMediaLibraryScan(t, svc, root.ID)
	if first.Status != domain.MediaScanStatusSucceeded || first.ImportedCount != 1 || first.FailedCount != 0 {
		t.Fatalf("unexpected first scan: %+v", first)
	}

	var music domain.Music
	if err := db.First(&music).Error; err != nil {
		t.Fatalf("load imported music: %v", err)
	}
	if music.Title != "Track" || music.Artist != "Artist" || music.Duration != 1 || !music.SourceReadOnly ||
		music.MediaRootID != root.ID || music.MediaRelativePath != "track.wav" {
		t.Fatalf("unexpected imported music: %+v", music)
	}
	originalHash := music.FileHash

	second := runMediaLibraryScan(t, svc, root.ID)
	if second.ExistingCount != 1 || second.ImportedCount != 0 {
		t.Fatalf("unchanged source was not incremental: %+v", second)
	}

	// #nosec G304 -- path is created inside this test's private temporary directory.
	file, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		t.Fatalf("open changed source: %v", err)
	}
	if _, err := file.Write([]byte("changed")); err != nil {
		_ = file.Close()
		t.Fatalf("change source: %v", err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("close changed source: %v", err)
	}
	third := runMediaLibraryScan(t, svc, root.ID)
	if third.SkippedCount != 1 || third.WarningCount != 1 || third.ImportedCount != 0 {
		t.Fatalf("changed source should require manual review: %+v", third)
	}
	var count int64
	if err := db.Model(&domain.Music{}).Count(&count).Error; err != nil || count != 1 {
		t.Fatalf("music count after repeated scans = %d, error = %v", count, err)
	}
	if err := db.First(&music).Error; err != nil {
		t.Fatalf("reload imported music: %v", err)
	}
	if music.FileHash != originalHash || music.Title != "Track" {
		t.Fatalf("changed source overwrote existing metadata or identity: %+v", music)
	}
}

func TestMediaLibraryRootRegistrationDoesNotProbeUnavailablePath(t *testing.T) {
	repo := &mediaLibraryRepositoryStub{}
	originalConfig := config.AppConfig
	managedDir := t.TempDir()
	config.AppConfig = &config.Config{Server: config.ServerConfig{UploadDir: managedDir}}
	t.Cleanup(func() { config.AppConfig = originalConfig })
	svc := NewMediaLibraryService(repo, &mediaLibraryMusicRepositoryStub{}, config.LibraryConfig{}).(*mediaLibraryService)
	unavailable := filepath.Join(t.TempDir(), "not-mounted")

	created, err := svc.CreateRoot(context.Background(), 7, &domain.CreateMediaLibraryRootRequest{Name: "NFS", Path: unavailable})
	if err != nil {
		t.Fatalf("register unavailable root without probing it: %v", err)
	}
	if created.Path != unavailable || !created.ReadOnly {
		t.Fatalf("unexpected root response: %+v", created)
	}
}

func TestMediaLibraryOfflineNFSUsesPersistedRetryState(t *testing.T) {
	db := openMediaLibraryTestDB(t, "offline-library.db",
		&domain.Music{}, &domain.MediaLibraryRoot{}, &domain.MediaLibraryRootState{},
		&domain.MediaFile{}, &domain.MediaScanJob{}, &domain.MediaScanIssue{},
	)

	libraryRepo := repository.NewMediaLibraryRepository(db)
	musicRepo := repository.NewMusicRepository(db)
	root := &domain.MediaLibraryRoot{
		Name: "Offline NFS", Path: filepath.Join(t.TempDir(), "not-mounted"), StorageKind: domain.MediaStorageKindNFS,
		Enabled: true, ReadOnly: true, CreatedBy: 1,
	}
	if err := libraryRepo.CreateRoot(context.Background(), root); err != nil {
		t.Fatalf("create NFS root: %v", err)
	}
	svc := NewMediaLibraryServiceWithProber(
		libraryRepo,
		musicRepo,
		config.LibraryConfig{Scanner: config.LibraryScannerConfig{
			Enabled: true, MaxFileSizeMB: 1, MaxTagSizeMB: 16,
			RetryMaxAttempts: 3, RetryInitialSeconds: 30, RetryMaxSeconds: 60,
		}},
		config.ServerConfig{UploadDir: t.TempDir(), MaxCoverSizeMB: 1},
		fixedMediaProber{result: mediafs.Result{
			Status: mediafs.StatusOffline, Code: "network_unreachable", Message: "NFS endpoint is unreachable", Retryable: true,
		}},
	).(*mediaLibraryService)

	job := runMediaLibraryScan(t, svc, root.ID)
	if job.Status != domain.MediaScanStatusRetryWait || job.Attempt != 1 || job.FailureCode != "network_unreachable" || !job.FailureRetryable {
		t.Fatalf("offline NFS should enter a structured retry state, got %+v", job)
	}
	if job.NextAttemptAt == nil || !job.NextAttemptAt.After(time.Now().UTC()) {
		t.Fatalf("offline NFS retry should have a future deadline, got %+v", job.NextAttemptAt)
	}
	state, err := libraryRepo.FindRootState(context.Background(), root.ID)
	if err != nil || state == nil || state.Code != "network_unreachable" || state.Status != domain.MediaRootHealthOffline {
		t.Fatalf("NFS health reason was not persisted: state=%+v err=%v", state, err)
	}
}

func TestReadOnlyPhysicalSourceProtectsHashLinkedTrack(t *testing.T) {
	db := openMediaLibraryTestDB(t, "source-policy.db", &domain.Music{}, &domain.MediaFile{})

	musicRepo := repository.NewMusicRepository(db)
	libraryRepo := repository.NewMediaLibraryRepository(db)
	music := &domain.Music{Title: "Owned upload", Artist: "Artist", UserID: 7, Type: domain.MusicTypeSingle}
	if err := musicRepo.Create(context.Background(), music); err != nil {
		t.Fatalf("create user-owned music: %v", err)
	}
	mediaFile := &domain.MediaFile{
		MusicID: music.ID, RootID: 9, RelativePath: "archive/song.flac", SourceKey: "readonly-source",
		ReadOnly: true, Availability: domain.MediaFileAvailabilityOnline,
	}
	if err := libraryRepo.CreateMediaFile(context.Background(), mediaFile); err != nil {
		t.Fatalf("link read-only physical source: %v", err)
	}

	libraryService := NewMediaLibraryServiceWithProber(
		libraryRepo,
		musicRepo,
		config.LibraryConfig{},
		config.ServerConfig{UploadDir: t.TempDir(), MaxCoverSizeMB: 1},
		fixedMediaProber{},
	)
	musicService := NewMusicServiceWithConfig(musicRepo, libraryService, &config.Config{
		Server: config.ServerConfig{UploadDir: t.TempDir(), MaxAudioSizeMB: 1, MaxCoverSizeMB: 1},
		Access: config.AccessConfig{LibraryMode: config.LibraryAccessPublic, MediaURLTTLMinutes: 60},
	})
	newTitle := "User rewrite"
	if _, err := musicService.Update(context.Background(), 7, "user", music.ID, &domain.UpdateMusicRequest{Title: &newTitle}); !errors.Is(err, ErrForbidden) {
		t.Fatalf("ordinary owner update error = %v, want forbidden", err)
	}
	if err := musicService.Delete(context.Background(), 7, "user", music.ID); !errors.Is(err, ErrForbidden) {
		t.Fatalf("ordinary owner delete error = %v, want forbidden", err)
	}

	stored, err := musicRepo.FindByID(context.Background(), music.ID)
	if err != nil || !stored.SourceReadOnly {
		t.Fatalf("read-only aggregate flag was not persisted: music=%+v err=%v", stored, err)
	}
}

func openMediaLibraryTestDB(t *testing.T, filename string, models ...any) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), filename)), &gorm.Config{})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get database handle: %v", err)
	}
	t.Cleanup(func() {
		if err := sqlDB.Close(); err != nil {
			t.Errorf("close database: %v", err)
		}
	})
	models = append(models,
		&domain.MusicArtistCredit{},
		&domain.MusicAlbumMembership{},
		&domain.MusicGenreFacet{},
	)
	if err := db.AutoMigrate(models...); err != nil {
		t.Fatalf("create media library schema: %v", err)
	}
	return db
}

type fixedMediaProber struct {
	result mediafs.Result
}

func (p fixedMediaProber) Probe(context.Context, mediafs.RootSpec) mediafs.Result {
	result := p.result
	result.CheckedAt = time.Now().UTC()
	return result
}

type mediaLibraryRepositoryStub struct {
	repository.MediaLibraryRepository
	roots []*domain.MediaLibraryRoot
}

func (s *mediaLibraryRepositoryStub) ListRoots(context.Context) ([]*domain.MediaLibraryRoot, error) {
	return s.roots, nil
}

func (s *mediaLibraryRepositoryStub) CreateRoot(_ context.Context, root *domain.MediaLibraryRoot) error {
	root.ID = uint(len(s.roots) + 1)
	s.roots = append(s.roots, root)
	return nil
}

func (s *mediaLibraryRepositoryStub) FindRootState(context.Context, uint) (*domain.MediaLibraryRootState, error) {
	return nil, nil
}

type mediaLibraryMusicRepositoryStub struct {
	repository.MusicRepository
}

func runMediaLibraryScan(t *testing.T, svc *mediaLibraryService, rootID uint) *domain.MediaScanJob {
	t.Helper()
	queued, err := svc.StartScan(context.Background(), rootID, 1)
	if err != nil {
		t.Fatalf("queue scan: %v", err)
	}
	claimed, found, err := svc.repo.ClaimNextScanJob(context.Background(), svc.workerID, mediaScanLeaseDuration)
	if err != nil || !found || claimed.ID != queued.ID {
		t.Fatalf("claim scan: job=%+v found=%v err=%v", claimed, found, err)
	}
	svc.runScanJob(context.Background(), claimed)
	finished, err := svc.repo.FindScanJob(context.Background(), claimed.ID)
	if err != nil {
		t.Fatalf("reload scan: %v", err)
	}
	return finished
}

func writeMinimalWAV(t *testing.T, path, title, artist string, dataBytes int) {
	t.Helper()
	infoBody := riffInfoBody(map[string]string{"INAM": title, "IART": artist})
	fmtBody := make([]byte, 16)
	binary.LittleEndian.PutUint16(fmtBody[0:2], 1)
	binary.LittleEndian.PutUint16(fmtBody[2:4], 1)
	binary.LittleEndian.PutUint32(fmtBody[4:8], 8000)
	binary.LittleEndian.PutUint32(fmtBody[8:12], 8000)
	binary.LittleEndian.PutUint16(fmtBody[12:14], 1)
	binary.LittleEndian.PutUint16(fmtBody[14:16], 8)

	content := []byte("RIFF\x00\x00\x00\x00WAVE")
	content = appendRIFFChunk(content, "fmt ", fmtBody)
	content = appendRIFFChunk(content, "LIST", infoBody)
	content = appendRIFFChunk(content, "data", make([]byte, dataBytes))
	binary.LittleEndian.PutUint32(content[4:8], checkedUint32Length(len(content)-8))
	if err := os.WriteFile(path, content, 0600); err != nil {
		t.Fatalf("write WAV: %v", err)
	}
}

func riffInfoBody(values map[string]string) []byte {
	body := []byte("INFO")
	for _, key := range []string{"INAM", "IART"} {
		value := append([]byte(values[key]), 0)
		body = appendRIFFChunk(body, key, value)
	}
	return body
}

func appendRIFFChunk(target []byte, key string, body []byte) []byte {
	target = append(target, []byte(key)...)
	var size [4]byte
	binary.LittleEndian.PutUint32(size[:], checkedUint32Length(len(body)))
	target = append(target, size[:]...)
	target = append(target, body...)
	if len(body)%2 == 1 {
		target = append(target, 0)
	}
	return target
}

func checkedUint32Length(value int) uint32 {
	if value < 0 || uint64(value) > math.MaxUint32 {
		panic("test RIFF chunk exceeds uint32 length")
	}
	return uint32(value)
}
