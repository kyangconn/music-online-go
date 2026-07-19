package service

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"mime/multipart"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/kyangconn/music-online-go/internal/config"
	"github.com/kyangconn/music-online-go/internal/domain"
	"github.com/kyangconn/music-online-go/internal/repository"
)

type uploadMusicRepositoryStub struct {
	repository.MusicRepository
	music     *domain.Music
	updateErr error
}

func (s *uploadMusicRepositoryStub) FindByID(_ context.Context, _ uint) (*domain.Music, error) {
	copy := *s.music
	return &copy, nil
}

func (s *uploadMusicRepositoryStub) Update(_ context.Context, music *domain.Music) error {
	if s.updateErr != nil {
		return s.updateErr
	}
	copy := *music
	s.music = &copy
	return nil
}

func TestUploadFilesRestoresExistingFilesWhenDatabaseUpdateFails(t *testing.T) {
	uploadDir := configureUploadTest(t)
	musicDir := filepath.Join(uploadDir, "1")
	if err := os.MkdirAll(musicDir, 0700); err != nil {
		t.Fatalf("create music directory: %v", err)
	}

	audioPath := filepath.Join(musicDir, "audio.mp3")
	coverPath := filepath.Join(musicDir, "cover.png")
	oldAudio := []byte("old audio")
	oldCover := []byte("old cover")
	writeTestFile(t, audioPath, oldAudio)
	writeTestFile(t, coverPath, oldCover)

	repo := &uploadMusicRepositoryStub{
		music:     &domain.Music{ID: 1, UserID: 7, Path: audioPath, Img: coverPath},
		updateErr: errors.New("database write failed"),
	}
	svc := NewMusicService(repo)
	newAudio := append([]byte("ID3\x04\x00\x00\x00\x00\x00\x10"), []byte("new audio")...)
	newCover := append([]byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'}, bytes.Repeat([]byte{0}, 24)...)

	_, err := svc.UploadFiles(
		context.Background(),
		7,
		"user",
		1,
		makeMultipartFileHeader(t, "file", "track.mp3", newAudio),
		makeMultipartFileHeader(t, "cover", "cover.png", newCover),
	)
	if err == nil || !bytes.Contains([]byte(err.Error()), []byte("database write failed")) {
		t.Fatalf("expected database error, got %v", err)
	}

	assertFileContent(t, audioPath, oldAudio)
	assertFileContent(t, coverPath, oldCover)
	assertNoStagingFiles(t, musicDir)
	if repo.music.Path != audioPath || repo.music.Img != coverPath {
		t.Fatalf("repository state changed after failed update: %#v", repo.music)
	}
}

func TestUploadFilesReplacesExistingFileOnWindowsCompatiblePath(t *testing.T) {
	uploadDir := configureUploadTest(t)
	musicDir := filepath.Join(uploadDir, "1")
	if err := os.MkdirAll(musicDir, 0700); err != nil {
		t.Fatalf("create music directory: %v", err)
	}

	audioPath := filepath.Join(musicDir, "audio.mp3")
	writeTestFile(t, audioPath, []byte("old audio"))
	repo := &uploadMusicRepositoryStub{music: &domain.Music{ID: 1, UserID: 7, Path: audioPath}}
	svc := NewMusicService(repo)
	newAudio := append([]byte("ID3\x04\x00\x00\x00\x00\x00\x10"), []byte("replacement")...)

	_, err := svc.UploadFiles(
		context.Background(),
		7,
		"user",
		1,
		makeMultipartFileHeader(t, "file", "track.mp3", newAudio),
		nil,
	)
	if err != nil {
		t.Fatalf("replace audio: %v", err)
	}

	assertFileContent(t, audioPath, newAudio)
	expectedHash := sha256.Sum256(newAudio)
	if repo.music.FileHash != hex.EncodeToString(expectedHash[:]) {
		t.Fatalf("file hash = %q, want %q", repo.music.FileHash, hex.EncodeToString(expectedHash[:]))
	}
	assertNoStagingFiles(t, musicDir)
}

func TestUploadFilesRemovesPreviousExtensionOnlyAfterCommit(t *testing.T) {
	uploadDir := configureUploadTest(t)
	musicDir := filepath.Join(uploadDir, "1")
	if err := os.MkdirAll(musicDir, 0700); err != nil {
		t.Fatalf("create music directory: %v", err)
	}

	oldPath := filepath.Join(musicDir, "audio.mp3")
	writeTestFile(t, oldPath, []byte("old audio"))
	repo := &uploadMusicRepositoryStub{music: &domain.Music{ID: 1, UserID: 7, Path: oldPath}}
	svc := NewMusicService(repo)
	newAudio := append([]byte("fLaC"), []byte("replacement")...)

	_, err := svc.UploadFiles(
		context.Background(),
		7,
		"user",
		1,
		makeMultipartFileHeader(t, "file", "track.flac", newAudio),
		nil,
	)
	if err != nil {
		t.Fatalf("replace audio extension: %v", err)
	}

	if _, err := os.Stat(oldPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("old extension still exists or stat failed: %v", err)
	}
	newPath := filepath.Join(musicDir, "audio.flac")
	assertFileContent(t, newPath, newAudio)
	if repo.music.Path != newPath {
		t.Fatalf("stored path = %q, want %q", repo.music.Path, newPath)
	}
	assertNoStagingFiles(t, musicDir)
}

func TestUploadFilesNeverDeletesReadOnlyLibrarySource(t *testing.T) {
	uploadDir := configureUploadTest(t)
	externalDir := t.TempDir()
	externalPath := filepath.Join(externalDir, "source.flac")
	externalContent := append([]byte("fLaC"), []byte("read-only source")...)
	writeTestFile(t, externalPath, externalContent)
	sourceKey := mediaSourceKey(12, "source.flac", domain.MediaPathSemanticsAuto)
	repo := &uploadMusicRepositoryStub{music: &domain.Music{
		ID: 1, UserID: 0, Path: externalPath, MediaRootID: 12,
		MediaRelativePath: "source.flac", MediaSourceKey: &sourceKey, SourceReadOnly: true,
	}}
	svc := NewMusicService(repo)
	replacement := append([]byte("fLaC"), []byte("managed replacement")...)

	if _, err := svc.UploadFiles(
		context.Background(),
		7,
		"admin",
		1,
		makeMultipartFileHeader(t, "file", "track.flac", replacement),
		nil,
	); err != nil {
		t.Fatalf("replace read-only source: %v", err)
	}

	assertFileContent(t, externalPath, externalContent)
	managedPath := filepath.Join(uploadDir, "1", "audio.flac")
	assertFileContent(t, managedPath, replacement)
	if repo.music.Path != managedPath || repo.music.MediaRootID != domain.ManagedMediaRootID ||
		repo.music.MediaRelativePath != "1/audio.flac" || repo.music.SourceReadOnly {
		t.Fatalf("replacement did not become a managed source: %#v", repo.music)
	}
}

func configureUploadTest(t *testing.T) string {
	t.Helper()
	originalConfig := config.AppConfig
	uploadDir := t.TempDir()
	config.AppConfig = &config.Config{Server: config.ServerConfig{
		MaxAudioSizeMB: 1,
		MaxCoverSizeMB: 1,
		UploadDir:      uploadDir,
	}}
	t.Cleanup(func() { config.AppConfig = originalConfig })
	return uploadDir
}

func makeMultipartFileHeader(t *testing.T, field, name string, content []byte) *multipart.FileHeader {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile(field, name)
	if err != nil {
		t.Fatalf("create multipart file: %v", err)
	}
	if _, err := part.Write(content); err != nil {
		t.Fatalf("write multipart file: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}

	req := httptest.NewRequest("POST", "/upload", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	if err := req.ParseMultipartForm(1 << 20); err != nil {
		t.Fatalf("parse multipart form: %v", err)
	}
	t.Cleanup(func() { _ = req.MultipartForm.RemoveAll() })
	return req.MultipartForm.File[field][0]
}

func writeTestFile(t *testing.T, path string, content []byte) {
	t.Helper()
	if err := os.WriteFile(path, content, 0600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func assertFileContent(t *testing.T, path string, expected []byte) {
	t.Helper()
	// #nosec G304 -- callers pass paths created under the test's temporary upload directory.
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if !bytes.Equal(content, expected) {
		t.Fatalf("content of %s = %q, want %q", path, content, expected)
	}
}

func assertNoStagingFiles(t *testing.T, dir string) {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read upload directory: %v", err)
	}
	for _, entry := range entries {
		if filepath.Ext(entry.Name()) == ".tmp" || filepath.Ext(entry.Name()) == ".backup" {
			t.Fatalf("staging file was not cleaned up: %s", entry.Name())
		}
	}
}
