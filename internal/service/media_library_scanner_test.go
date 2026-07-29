package service

import (
	"context"
	"errors"
	"testing"

	"github.com/kyangconn/music-online-go/internal/domain"
	"github.com/kyangconn/music-online-go/internal/repository"
)

func TestApplyScanFileResultOwnsCountersAndIssues(t *testing.T) {
	repo := &scanIssueRepositoryStub{}
	service := &mediaLibraryService{repo: repo}
	job := &domain.MediaScanJob{ID: 7}

	result := scanFileResult{
		disposition: scanFileImported,
		issues: []scanFileIssue{
			{severity: "warning", code: "metadata_unreadable", message: "fallbacks used"},
			{severity: "warning", code: "cover_store_failed", message: "cover skipped"},
		},
	}
	if err := service.applyScanFileResult(context.Background(), job, "album/track.flac", result); err != nil {
		t.Fatalf("apply result: %v", err)
	}
	if job.ImportedCount != 1 || job.WarningCount != 2 || job.FailedCount != 0 {
		t.Fatalf("unexpected counters: %+v", job)
	}
	if len(repo.issues) != 2 || repo.issues[0].RelativePath != "album/track.flac" {
		t.Fatalf("persisted issues = %+v", repo.issues)
	}
}

func TestApplyScanFileResultPropagatesStopSignal(t *testing.T) {
	stopErr := errors.New("lease lost")
	service := &mediaLibraryService{repo: &scanIssueRepositoryStub{}}
	job := &domain.MediaScanJob{ID: 8}

	err := service.applyScanFileResult(context.Background(), job, "track.flac", scanFileResult{stopErr: stopErr})
	if !errors.Is(err, stopErr) {
		t.Fatalf("stop error = %v, want %v", err, stopErr)
	}
	if job.ProcessedCount != 0 || job.FailedCount != 0 || job.WarningCount != 0 {
		t.Fatalf("stop-only result changed counters: %+v", job)
	}
}

type scanIssueRepositoryStub struct {
	repository.MediaLibraryRepository
	issues []*domain.MediaScanIssue
}

func (repo *scanIssueRepositoryStub) CreateScanIssue(_ context.Context, issue *domain.MediaScanIssue) error {
	repo.issues = append(repo.issues, issue)
	return nil
}
