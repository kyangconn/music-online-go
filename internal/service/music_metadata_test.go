package service

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/kyangconn/music-online-go/internal/domain"
)

func TestNormalizeMusicMetadataBoundsCompatibilityScalars(t *testing.T) {
	music := &domain.Music{
		Title: "Track", Artist: "Artist",
		AlbumArtists: domain.StringList{strings.Repeat("a", 200), strings.Repeat("b", 200)},
		Genres:       domain.StringList{strings.Repeat("c", 300), strings.Repeat("d", 300)},
	}
	if err := normalizeMusicMetadata(music); err != nil {
		t.Fatalf("normalize bounded multi-value metadata: %v", err)
	}
	if len(music.AlbumArtists) != 2 || len(music.Genres) != 2 {
		t.Fatalf("lossless multi-value fields were truncated: %+v", music)
	}
	if len(music.AlbumArtist) > maxCanonicalArtistBytes || len(music.Genre) > maxCanonicalGenreBytes {
		t.Fatalf("compatibility scalar exceeded its database bound: %+v", music)
	}
}

func TestNormalizeMusicMetadataRejectsExcessValues(t *testing.T) {
	values := make(domain.StringList, maxCanonicalMetadataValues+1)
	for index := range values {
		values[index] = fmt.Sprintf("artist-%d", index)
	}
	music := &domain.Music{Title: "Track", Artist: "Artist", Artists: values}
	if err := normalizeMusicMetadata(music); err == nil {
		t.Fatal("an excessive metadata value count should be rejected")
	}
}

func TestApplyCreateMusicMetadataPreservesDisplayValuesAndBuildsM5Inputs(t *testing.T) {
	music := &domain.Music{}
	req := &domain.CreateMusicRequest{
		Title:                  "  A Track  ",
		Artist:                 "Artist feat. Guest",
		Artists:                domain.StringList{"Artist", "Guest", "artist"},
		AlbumArtist:            "Album Artist",
		TrackNumber:            2,
		TrackTotal:             12,
		DiscNumber:             1,
		DiscTotal:              2,
		ReleaseDate:            "2024-03",
		OriginalReleaseDate:    "2023",
		Genres:                 domain.StringList{"Ambient / Chillout", "AMBIENT"},
		ISRCs:                  domain.StringList{" us-abc-24-12345 "},
		MusicBrainzRecordingID: "123E4567-E89B-42D3-A456-426614174000",
		MusicBrainzArtistIDs:   domain.StringList{"123e4567-e89b-42d3-a456-426614174001"},
	}

	if err := applyCreateMusicMetadata(music, req); err != nil {
		t.Fatalf("apply metadata: %v", err)
	}
	if music.Title != "A Track" || music.Artist != "Artist feat. Guest" {
		t.Fatalf("display values were not preserved: %q / %q", music.Title, music.Artist)
	}
	if got, want := music.Artists, (domain.StringList{"Artist", "Guest"}); !reflect.DeepEqual(got, want) {
		t.Fatalf("artists = %#v, want %#v", got, want)
	}
	if got, want := music.Genres, (domain.StringList{"Ambient / Chillout", "AMBIENT"}); !reflect.DeepEqual(got, want) {
		t.Fatalf("genres = %#v, want %#v", got, want)
	}
	if got, want := music.GenreTokens, (domain.StringList{"ambient", "chillout"}); !reflect.DeepEqual(got, want) {
		t.Fatalf("genre tokens = %#v, want %#v", got, want)
	}
	if music.MetadataRevision != 1 || music.Year != 2024 {
		t.Fatalf("revision/year = %d/%d", music.MetadataRevision, music.Year)
	}
	if got := music.ISRCs[0]; got != "USABC2412345" {
		t.Fatalf("ISRC = %q", got)
	}
	if music.MusicBrainzRecordingID != "123e4567-e89b-42d3-a456-426614174000" {
		t.Fatalf("recording ID = %q", music.MusicBrainzRecordingID)
	}
}

func TestApplyUpdateMusicMetadataAdvancesRevisionOnlyForCanonicalMetadata(t *testing.T) {
	music := &domain.Music{Title: "Track", Artist: "Artist", MetadataRevision: 4}
	intro := "new description"
	changed, err := applyUpdateMusicMetadata(music, &domain.UpdateMusicRequest{Intro: &intro})
	if err != nil || changed || music.MetadataRevision != 4 {
		t.Fatalf("non-metadata update changed revision: changed=%v revision=%d err=%v", changed, music.MetadataRevision, err)
	}

	comment := "new tag comment"
	changed, err = applyUpdateMusicMetadata(music, &domain.UpdateMusicRequest{Comment: &comment})
	if err != nil || !changed || music.MetadataRevision != 5 {
		t.Fatalf("metadata update revision: changed=%v revision=%d err=%v", changed, music.MetadataRevision, err)
	}
}

func TestMusicMetadataRejectsAmbiguousOrInvalidValues(t *testing.T) {
	tests := []domain.CreateMusicRequest{
		{Title: "", Artist: "Artist"},
		{Title: "Track", Artist: "Artist", TrackNumber: 3, TrackTotal: 2},
		{Title: "Track", Artist: "Artist", ReleaseDate: "2024-13"},
		{Title: "Track", Artist: "Artist", ISRCs: domain.StringList{"not-an-isrc"}},
		{Title: "Track", Artist: "Artist", MusicBrainzRecordingID: "artist-id-is-not-a-recording-id"},
	}
	for _, req := range tests {
		if err := applyCreateMusicMetadata(&domain.Music{}, &req); !errors.Is(err, ErrInvalidMusicMetadata) {
			t.Errorf("request %+v error = %v, want ErrInvalidMusicMetadata", req, err)
		}
	}
}

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
