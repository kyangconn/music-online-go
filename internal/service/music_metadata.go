package service

import (
	"github.com/kyangconn/music-online-go/internal/domain"
	"github.com/kyangconn/music-online-go/internal/musicmeta"
)

var ErrInvalidMusicMetadata = musicmeta.ErrInvalidMusicMetadata

// Compatibility forwards used by the tag reader until it moves into its own
// package in the next refactor step.
const maxCanonicalMetadataValues = musicmeta.MaxCanonicalMetadataValues

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

func normalizeDisplayValues(values domain.StringList, split bool) domain.StringList {
	return musicmeta.NormalizeDisplayValues(values, split)
}

func joinMetadataDisplayValues(values domain.StringList, maxBytes int) string {
	return musicmeta.JoinDisplayValues(values, maxBytes)
}

func normalizeISRCs(values domain.StringList) (domain.StringList, error) {
	return musicmeta.NormalizeISRCs(values)
}

func validatePartialDate(field, value string) error {
	return musicmeta.ValidatePartialDate(field, value)
}
