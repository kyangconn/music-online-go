package service

import (
	"fmt"
	"net/url"
	"time"

	"github.com/kyangconn/music-online-go/internal/config"
	"github.com/kyangconn/music-online-go/internal/domain"
	"github.com/kyangconn/music-online-go/internal/pkg/mediatoken"
)

// musicPresenter is the single place that converts physical media presence
// into public API URLs. Browse pages and ordinary music responses therefore use
// identical private-library token and expiry semantics.
type musicPresenter struct {
	access config.AccessConfig
	jwt    config.JWTConfig
	now    func() time.Time
}

func newMusicPresenter(cfg *config.Config) musicPresenter {
	access := config.AccessConfig{LibraryMode: config.LibraryAccessPublic, MediaURLTTLMinutes: 60}
	jwtConfig := config.JWTConfig{}
	if cfg != nil {
		access = cfg.Access
		jwtConfig = cfg.JWT
	}
	return musicPresenter{access: access, jwt: jwtConfig, now: time.Now}
}

func (p musicPresenter) music(music *domain.Music) *domain.MusicResponse {
	response := music.ToResponse()
	if response == nil || p.access.LibraryMode != config.LibraryAccessAuthenticated {
		return response
	}
	expiresAt := p.expiresAt()
	if response.Path != "" {
		response.Path = p.sign(response.Path, music.ID, "stream", expiresAt)
	}
	if response.Img != "" {
		response.Img = p.sign(response.Img, music.ID, "cover", expiresAt)
	}
	if response.Path != "" || response.Img != "" {
		response.MediaURLExpiresAt = &expiresAt
	}
	return response
}

func (p musicPresenter) cover(musicID uint) (string, *time.Time) {
	path := fmt.Sprintf("/api/v1/musics/%d/cover", musicID)
	if p.access.LibraryMode != config.LibraryAccessAuthenticated {
		return path, nil
	}
	expiresAt := p.expiresAt()
	return p.sign(path, musicID, "cover", expiresAt), &expiresAt
}

func (p musicPresenter) expiresAt() time.Time {
	return p.now().UTC().Add(time.Duration(p.access.MediaURLTTLMinutes) * time.Minute).Truncate(time.Second)
}

func (p musicPresenter) sign(path string, musicID uint, scope string, expiresAt time.Time) string {
	token := mediatoken.Issue(p.jwt.Secret, musicID, scope, expiresAt)
	return path + "?media_token=" + url.QueryEscape(token)
}
