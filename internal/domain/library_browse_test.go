package domain

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
)

func TestBuildMusicBrowseProjectionPrefersStableIDsAndPreservesDisplayNames(t *testing.T) {
	music := &Music{
		ID: 7, Artist: "Artist feat. Guest", Artists: StringList{"Artist", "Guest"},
		MusicBrainzArtistIDs: StringList{
			"123e4567-e89b-42d3-a456-426614174001",
			"123e4567-e89b-42d3-a456-426614174002",
		},
		Album: "Release", AlbumArtist: "Artist", AlbumArtists: StringList{"Artist"},
		MusicBrainzAlbumArtistIDs: StringList{"123e4567-e89b-42d3-a456-426614174001"},
		MusicBrainzReleaseID:      "123e4567-e89b-42d3-a456-426614174003",
		Genres:                    StringList{"Drum & Bass / Electronic", "electronic"}, Img: "cover.jpg",
	}

	projection := BuildMusicBrowseProjection(music)
	if len(projection.ArtistCredits) != 2 {
		t.Fatalf("artist credits = %#v", projection.ArtistCredits)
	}
	if got := projection.ArtistCredits[0].GroupKey; got != "mbid_123e4567-e89b-42d3-a456-426614174001" {
		t.Fatalf("primary artist key = %q", got)
	}
	if !projection.ArtistCredits[0].TrackCredit || !projection.ArtistCredits[0].AlbumCredit ||
		!projection.ArtistCredits[0].HasCover || projection.ArtistCredits[0].Name != "Artist" ||
		len(projection.ArtistCredits[0].NormalizedNameKey) != 64 {
		t.Fatalf("primary artist projection = %+v", projection.ArtistCredits[0])
	}
	if projection.AlbumMembership == nil || projection.AlbumMembership.GroupKey != "mbid_123e4567-e89b-42d3-a456-426614174003" {
		t.Fatalf("album projection = %+v", projection.AlbumMembership)
	}
	if got := NormalizeGenreTokens(music.Genres); !reflect.DeepEqual(got, StringList{"drum and bass", "electronic"}) {
		t.Fatalf("genre tokens = %#v", got)
	}
}

func TestBrowseFallbackKeysFoldUnicodeAndKeepDifferentEntitiesSeparate(t *testing.T) {
	wide := BrowseGroupKey("", " ＡＲＴＩＳＴ  Name ")
	plain := BrowseGroupKey("", "artist name")
	other := BrowseGroupKey("", "artist names")
	if wide != plain {
		t.Fatalf("compatibility-equivalent names produced different keys: %q != %q", wide, plain)
	}
	if plain == other {
		t.Fatal("different names produced the same fallback key")
	}
	if !IsBrowseGroupKey(plain) || IsBrowseGroupKey("text_not-a-hash") {
		t.Fatalf("browse key validation mismatch: valid=%v invalid=%v", IsBrowseGroupKey(plain), IsBrowseGroupKey("text_not-a-hash"))
	}
}

func TestAlbumFallbackIncludesAlbumArtistAndOmitsUntaggedAlbums(t *testing.T) {
	first := &Music{ID: 1, Artist: "Track Artist", Album: "Shared", AlbumArtist: "Artist A"}
	second := &Music{ID: 2, Artist: "Track Artist", Album: "shared", AlbumArtist: "artist a"}
	third := &Music{ID: 3, Artist: "Track Artist", Album: "Shared", AlbumArtist: "Artist B"}
	untagged := &Music{ID: 4, Artist: "Track Artist"}
	releaseGroupOnly := &Music{
		ID: 5, Artist: "Track Artist", Album: "Shared", AlbumArtist: "Artist A",
		MusicBrainzReleaseGroupID: "123e4567-e89b-42d3-a456-426614174099",
	}

	firstProjection := BuildMusicBrowseProjection(first)
	secondProjection := BuildMusicBrowseProjection(second)
	thirdProjection := BuildMusicBrowseProjection(third)
	if firstProjection.AlbumMembership == nil || secondProjection.AlbumMembership == nil || thirdProjection.AlbumMembership == nil {
		t.Fatal("tagged tracks should have album memberships")
	}
	if firstProjection.AlbumMembership.GroupKey != secondProjection.AlbumMembership.GroupKey {
		t.Fatal("case-only album variants should share a fallback identity")
	}
	if firstProjection.AlbumMembership.GroupKey == thirdProjection.AlbumMembership.GroupKey {
		t.Fatal("same-name albums by different album artists must remain separate")
	}
	if BuildMusicBrowseProjection(untagged).AlbumMembership != nil {
		t.Fatal("track without an album tag must not create an unknown-album entity")
	}
	if key := BuildMusicBrowseProjection(releaseGroupOnly).AlbumMembership.GroupKey; key != "mbid_123e4567-e89b-42d3-a456-426614174099" {
		t.Fatalf("release-group fallback key = %q", key)
	}
}

func TestBrowseProjectionBoundsUntrustedFacetsAndRejectsMalformedStableIDs(t *testing.T) {
	genres := make(StringList, 0, maxMusicGenreFacets+20)
	for index := 0; index < maxMusicGenreFacets+20; index++ {
		genres = append(genres, fmt.Sprintf("Genre %d", index))
	}
	music := &Music{
		ID: 9, Artist: "Artist", Artists: StringList{"Artist"},
		MusicBrainzArtistIDs: StringList{"not-an-mbid"}, Genres: genres,
	}
	projection := BuildMusicBrowseProjection(music)
	if len(projection.GenreFacets) != maxMusicGenreFacets {
		t.Fatalf("genre facet count = %d, want %d", len(projection.GenreFacets), maxMusicGenreFacets)
	}
	if len(projection.ArtistCredits) != 1 || !strings.HasPrefix(projection.ArtistCredits[0].GroupKey, "text_") ||
		projection.ArtistCredits[0].MusicBrainzArtistID != "" {
		t.Fatalf("malformed stable ID escaped into projection: %+v", projection.ArtistCredits)
	}
}

func TestAlbumCreditReusesUnambiguousTrackArtistIdentity(t *testing.T) {
	artistID := "123e4567-e89b-42d3-a456-426614174088"
	music := &Music{
		ID: 10, Artist: "Artist", Artists: StringList{"Artist"}, MusicBrainzArtistIDs: StringList{artistID},
		Album: "Album", AlbumArtist: "artist", AlbumArtists: StringList{"artist"},
	}
	projection := BuildMusicBrowseProjection(music)
	if len(projection.ArtistCredits) != 1 || !projection.ArtistCredits[0].TrackCredit || !projection.ArtistCredits[0].AlbumCredit {
		t.Fatalf("artist credits = %+v", projection.ArtistCredits)
	}
	if projection.AlbumMembership == nil || projection.AlbumMembership.AlbumArtistKey != "mbid_"+artistID {
		t.Fatalf("album artist identity = %+v", projection.AlbumMembership)
	}
}
