package service

import (
	"github.com/kyangconn/music-online-go/internal/domain"
	"github.com/kyangconn/music-online-go/internal/musicmeta"
)

var ErrInvalidMusicMetadata = musicmeta.ErrInvalidMusicMetadata

func applyCreateMusicMetadata(music *domain.Music, req *domain.CreateMusicRequest) error {
	return musicmeta.ApplyCreate(music, req)
}

func applyUpdateMusicMetadata(music *domain.Music, req *domain.UpdateMusicRequest) (bool, error) {
	return musicmeta.ApplyUpdate(music, req)
}

func normalizeMBID(field, value string) (string, error) {
	return musicmeta.NormalizeMBID(field, value)
}

func musicMetadataFromMusic(music *domain.Music) domain.MusicMetadata {
	return musicmeta.FromMusic(music)
}
