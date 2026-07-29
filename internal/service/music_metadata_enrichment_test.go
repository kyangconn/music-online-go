package service

import (
	"testing"

	"github.com/kyangconn/music-online-go/internal/domain"
)

func TestMetadataEnrichmentNeverUsesArtistIDAsTrackIdentity(t *testing.T) {
	existing := &domain.Music{Title: "Track", Artist: "Artist", MusicBrainzArtistIDs: domain.StringList{"123e4567-e89b-42d3-a456-426614174001"}}
	incoming := domain.MusicMetadata{
		Title:                  "Different Track",
		Artist:                 "Artist",
		MusicBrainzArtistIDs:   domain.StringList{"123e4567-e89b-42d3-a456-426614174001"},
		MusicBrainzRecordingID: "123e4567-e89b-42d3-a456-426614174000",
	}
	patch := buildMetadataEnrichment(existing, incoming)
	if patch == nil || patch.MusicBrainzRecordingID == nil {
		t.Fatal("blank recording ID should be enrichable without treating the artist ID as track identity")
	}
}
