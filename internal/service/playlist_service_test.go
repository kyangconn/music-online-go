package service

import (
	"errors"
	"strings"
	"testing"
)

func TestPlaylistTextValidationUsesCharactersAndNormalizesNames(t *testing.T) {
	name, err := normalizePlaylistName("  我的   播放列表  ")
	if err != nil || name != "我的 播放列表" {
		t.Fatalf("normalized name = %q, err=%v", name, err)
	}
	if _, err := normalizePlaylistName(strings.Repeat("乐", maxPlaylistNameCharacters)); err != nil {
		t.Fatalf("multibyte name at limit: %v", err)
	}
	if _, err := normalizePlaylistName(strings.Repeat("乐", maxPlaylistNameCharacters+1)); !errors.Is(err, ErrInvalidPlaylist) {
		t.Fatalf("overlong name error = %v", err)
	}
	if _, err := normalizePlaylistDescription(strings.Repeat("音", maxPlaylistDescriptionCharacters+1)); !errors.Is(err, ErrInvalidPlaylist) {
		t.Fatalf("overlong description error = %v", err)
	}
}
