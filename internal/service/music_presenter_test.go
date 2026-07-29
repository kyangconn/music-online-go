package service

import (
	"net/url"
	"testing"
	"time"

	"github.com/kyangconn/music-online-go/internal/config"
	"github.com/kyangconn/music-online-go/internal/domain"
	"github.com/kyangconn/music-online-go/internal/pkg/mediatoken"
)

func TestMusicPresenterSharesPrivateTokenSemanticsWithBrowseCovers(t *testing.T) {
	now := time.Date(2026, time.July, 26, 12, 0, 0, 0, time.UTC)
	presenter := newMusicPresenter(
		config.AccessConfig{LibraryMode: config.LibraryAccessAuthenticated, MediaURLTTLMinutes: 15},
		config.JWTConfig{Secret: "private-media-test-secret"},
	)
	presenter.now = func() time.Time { return now }

	response := presenter.music(&domain.Music{ID: 42, Title: "Track", Artist: "Artist", Path: "audio.flac", Img: "cover.jpg"})
	if response.MediaURLExpiresAt == nil || !response.MediaURLExpiresAt.Equal(now.Add(15*time.Minute)) {
		t.Fatalf("music expiry = %v", response.MediaURLExpiresAt)
	}
	assertPresentedMediaToken(t, response.Path, "private-media-test-secret", 42, "stream", now)
	assertPresentedMediaToken(t, response.Img, "private-media-test-secret", 42, "cover", now)

	coverURL, expiresAt := presenter.cover(42)
	if expiresAt == nil || !expiresAt.Equal(*response.MediaURLExpiresAt) {
		t.Fatalf("browse cover expiry = %v, music expiry = %v", expiresAt, response.MediaURLExpiresAt)
	}
	assertPresentedMediaToken(t, coverURL, "private-media-test-secret", 42, "cover", now)
}

func TestMusicPresenterLeavesPublicMediaURLsUnsigned(t *testing.T) {
	defaults := config.DefaultConfig()
	presenter := newMusicPresenter(defaults.Access, defaults.JWT)
	response := presenter.music(&domain.Music{ID: 7, Title: "Track", Artist: "Artist", Path: "audio.flac", Img: "cover.jpg"})
	if response.Path != "/api/v1/musics/7/stream" || response.Img != "/api/v1/musics/7/cover" || response.MediaURLExpiresAt != nil {
		t.Fatalf("public music response = %+v", response)
	}
	coverURL, expiresAt := presenter.cover(7)
	if coverURL != "/api/v1/musics/7/cover" || expiresAt != nil {
		t.Fatalf("public browse cover = %q, expiry=%v", coverURL, expiresAt)
	}
}

func assertPresentedMediaToken(t *testing.T, rawURL, secret string, musicID uint, scope string, now time.Time) {
	t.Helper()
	parsed, err := url.Parse(rawURL)
	if err != nil {
		t.Fatalf("parse presented URL %q: %v", rawURL, err)
	}
	if err := mediatoken.Validate(parsed.Query().Get("media_token"), secret, musicID, scope, now); err != nil {
		t.Fatalf("validate %s token in %q: %v", scope, rawURL, err)
	}
}
