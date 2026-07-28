package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"regexp"
	"sort"
	"strings"
	"time"

	"golang.org/x/text/cases"
	"golang.org/x/text/unicode/norm"
)

const (
	ArtistCreditTrack   = "track"
	ArtistCreditAlbum   = "album"
	maxMusicGenreFacets = 256
	maxGenreFacetBytes  = 500
)

var genreFacetSeparator = regexp.MustCompile(`[;,/]+`)
var browseStableIDPattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
var browseGroupKeyPattern = regexp.MustCompile(`^(?:mbid_[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}|text_[0-9a-f]{64})$`)

// MusicArtistCredit is a rebuildable projection of a track's credited artists.
// GroupKey prefers a MusicBrainz artist ID and otherwise hashes normalized text,
// while Name always preserves a display value from the source metadata.
type MusicArtistCredit struct {
	MusicID             uint   `json:"-" gorm:"primaryKey;index:idx_artist_credit_group_music,priority:2"`
	GroupKey            string `json:"key" gorm:"primaryKey;size:80;index:idx_artist_credit_group_music,priority:1;index"`
	Name                string `json:"name" gorm:"size:255;not null"`
	NormalizedName      string `json:"-" gorm:"type:text;not null"`
	NormalizedNameKey   string `json:"-" gorm:"size:64;not null;index"`
	MusicBrainzArtistID string `json:"musicbrainz_artist_id" gorm:"size:36;index"`
	TrackCredit         bool   `json:"-" gorm:"not null;default:false"`
	AlbumCredit         bool   `json:"-" gorm:"not null;default:false"`
	Position            int    `json:"-" gorm:"not null;default:0"`
	HasCover            bool   `json:"-" gorm:"not null;default:false"`
	Music               Music  `json:"-" gorm:"foreignKey:MusicID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
}

func (*MusicArtistCredit) TableName() string {
	return "music_artist_credits"
}

// MusicAlbumMembership stores the portable grouping and ordering fields for a
// track that belongs to an album. It is derived from Music and never becomes a
// second editable metadata source.
type MusicAlbumMembership struct {
	MusicID                   uint   `json:"-" gorm:"primaryKey"`
	GroupKey                  string `json:"key" gorm:"size:80;not null;index"`
	Title                     string `json:"title" gorm:"size:255;not null"`
	NormalizedTitle           string `json:"-" gorm:"type:text;not null"`
	NormalizedTitleKey        string `json:"-" gorm:"size:64;not null;index"`
	AlbumArtist               string `json:"album_artist" gorm:"size:255"`
	NormalizedAlbumArtist     string `json:"-" gorm:"type:text"`
	NormalizedAlbumArtistKey  string `json:"-" gorm:"size:64;not null;index"`
	AlbumArtistKey            string `json:"-" gorm:"size:80;index"`
	MusicBrainzReleaseID      string `json:"musicbrainz_release_id" gorm:"size:36;index"`
	MusicBrainzReleaseGroupID string `json:"musicbrainz_release_group_id" gorm:"size:36;index"`
	Year                      int    `json:"year" gorm:"index"`
	DiscNumber                int    `json:"-"`
	TrackNumber               int    `json:"-"`
	Duration                  int    `json:"-"`
	HasCover                  bool   `json:"-" gorm:"not null;default:false"`
	Music                     Music  `json:"-" gorm:"foreignKey:MusicID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
}

func (*MusicAlbumMembership) TableName() string {
	return "music_album_memberships"
}

// MusicGenreFacet gives exact, index-backed genre filtering without relying on
// database-specific JSON operators. DisplayName remains suitable for the UI;
// NormalizedName is the stable comparison token shared with future M5 rules.
type MusicGenreFacet struct {
	MusicID        uint   `json:"-" gorm:"primaryKey;index:idx_genre_facet_name_music,priority:2"`
	NormalizedName string `json:"value" gorm:"primaryKey;size:500;index:idx_genre_facet_name_music,priority:1"`
	DisplayName    string `json:"label" gorm:"size:500;not null"`
	Position       int    `json:"-" gorm:"not null;default:0"`
	Music          Music  `json:"-" gorm:"foreignKey:MusicID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
}

func (*MusicGenreFacet) TableName() string {
	return "music_genre_facets"
}

// MusicBrowseProjection contains every derived row that must be replaced in
// the same transaction whenever canonical Music metadata changes.
type MusicBrowseProjection struct {
	ArtistCredits   []MusicArtistCredit
	AlbumMembership *MusicAlbumMembership
	GenreFacets     []MusicGenreFacet
}

type ArtistSummary struct {
	Key                 string     `json:"key"`
	Name                string     `json:"name"`
	MusicBrainzArtistID string     `json:"musicbrainz_artist_id"`
	TrackCount          int64      `json:"track_count"`
	AlbumCount          int64      `json:"album_count"`
	CoverURL            string     `json:"cover_url,omitempty"`
	CoverURLExpiresAt   *time.Time `json:"cover_url_expires_at,omitempty"`
	CoverMusicID        *uint      `json:"-"`
}

type AlbumSummary struct {
	Key                       string     `json:"key"`
	Title                     string     `json:"title"`
	AlbumArtist               string     `json:"album_artist"`
	AlbumArtistKey            string     `json:"album_artist_key,omitempty"`
	MusicBrainzReleaseID      string     `json:"musicbrainz_release_id"`
	MusicBrainzReleaseGroupID string     `json:"musicbrainz_release_group_id"`
	Year                      int        `json:"year"`
	TrackCount                int64      `json:"track_count"`
	TotalDuration             int64      `json:"total_duration"`
	DiscCount                 int64      `json:"disc_count"`
	CoverURL                  string     `json:"cover_url,omitempty"`
	CoverURLExpiresAt         *time.Time `json:"cover_url_expires_at,omitempty"`
	CoverMusicID              *uint      `json:"-"`
}

type BrowseArtistParams struct {
	Query    string
	Page     int
	PageSize int
}

type BrowseAlbumParams struct {
	Query       string
	ArtistKey   string
	AlbumArtist string
	Genre       string
	Year        *int
	Page        int
	PageSize    int
}

// BuildMusicBrowseProjection deterministically derives browse identities from
// canonical metadata. It deliberately contains no database calls so migrations,
// normal writes and tests all use exactly the same grouping rules.
func BuildMusicBrowseProjection(music *Music) MusicBrowseProjection {
	projection := MusicBrowseProjection{
		ArtistCredits: make([]MusicArtistCredit, 0),
		GenreFacets:   make([]MusicGenreFacet, 0),
	}
	if music == nil || music.ID == 0 {
		return projection
	}
	projection.GenreFacets = BuildMusicGenreFacets(music)

	type pendingArtistCredit struct {
		display    string
		normalized string
		role       string
		position   int
	}
	credits := make(map[string]*MusicArtistCredit)
	stableKeysByName := make(map[string]map[string]struct{})
	pending := make([]pendingArtistCredit, 0)
	addCredit := func(key, stableID string, item pendingArtistCredit) {
		credit, exists := credits[key]
		if !exists {
			credit = &MusicArtistCredit{
				MusicID: music.ID, GroupKey: key, Name: item.display, NormalizedName: item.normalized,
				NormalizedNameKey: NormalizedBrowseTextKey(item.normalized), MusicBrainzArtistID: stableID,
				Position: item.position, HasCover: music.Img != "",
			}
			credits[key] = credit
		} else if item.position < credit.Position {
			credit.Position = item.position
		}
		if item.role == ArtistCreditTrack {
			credit.TrackCredit = true
		} else {
			credit.AlbumCredit = true
		}
	}
	appendCredits := func(names, ids StringList, fallback string, role string) {
		if len(names) == 0 && strings.TrimSpace(fallback) != "" {
			names = StringList{fallback}
		}
		for position, name := range names {
			display := strings.TrimSpace(name)
			normalized := NormalizeBrowseText(display)
			if normalized == "" {
				continue
			}
			stableID := ""
			if position < len(ids) {
				stableID = normalizeBrowseStableID(ids[position])
			}
			item := pendingArtistCredit{display: display, normalized: normalized, role: role, position: position}
			if stableID == "" {
				pending = append(pending, item)
				continue
			}
			key := BrowseGroupKey(stableID, normalized)
			addCredit(key, stableID, item)
			if stableKeysByName[normalized] == nil {
				stableKeysByName[normalized] = make(map[string]struct{})
			}
			stableKeysByName[normalized][key] = struct{}{}
		}
	}
	appendCredits(music.Artists, music.MusicBrainzArtistIDs, music.Artist, ArtistCreditTrack)
	appendCredits(music.AlbumArtists, music.MusicBrainzAlbumArtistIDs, music.AlbumArtist, ArtistCreditAlbum)
	for _, item := range pending {
		key := BrowseGroupKey("", item.normalized)
		if stableKeys := stableKeysByName[item.normalized]; len(stableKeys) == 1 {
			for stableKey := range stableKeys {
				key = stableKey
			}
		}
		addCredit(key, "", item)
	}
	for _, credit := range credits {
		projection.ArtistCredits = append(projection.ArtistCredits, *credit)
	}
	sort.Slice(projection.ArtistCredits, func(left, right int) bool {
		if projection.ArtistCredits[left].Position != projection.ArtistCredits[right].Position {
			return projection.ArtistCredits[left].Position < projection.ArtistCredits[right].Position
		}
		return projection.ArtistCredits[left].GroupKey < projection.ArtistCredits[right].GroupKey
	})

	if album := strings.TrimSpace(music.Album); album != "" {
		albumArtist := strings.TrimSpace(music.AlbumArtist)
		if albumArtist == "" {
			albumArtist = strings.TrimSpace(music.Artist)
		}
		albumArtistID := ""
		if len(music.MusicBrainzAlbumArtistIDs) > 0 {
			albumArtistID = normalizeBrowseStableID(music.MusicBrainzAlbumArtistIDs[0])
		}
		releaseID := normalizeBrowseStableID(music.MusicBrainzReleaseID)
		releaseGroupID := normalizeBrowseStableID(music.MusicBrainzReleaseGroupID)
		albumStableID := releaseID
		if albumStableID == "" {
			albumStableID = releaseGroupID
		}
		normalizedAlbumArtist := NormalizeBrowseText(albumArtist)
		albumArtistKey := BrowseGroupKey(albumArtistID, normalizedAlbumArtist)
		if albumArtistID == "" {
			if stableKeys := stableKeysByName[normalizedAlbumArtist]; len(stableKeys) == 1 {
				for stableKey := range stableKeys {
					albumArtistKey = stableKey
				}
			}
		}
		fallback := normalizedAlbumArtist + "\x00" + NormalizeBrowseText(album)
		projection.AlbumMembership = &MusicAlbumMembership{
			MusicID: music.ID, GroupKey: BrowseGroupKey(albumStableID, fallback),
			Title: album, NormalizedTitle: NormalizeBrowseText(album), NormalizedTitleKey: NormalizedBrowseTextKey(album),
			AlbumArtist:              albumArtist,
			NormalizedAlbumArtist:    normalizedAlbumArtist,
			NormalizedAlbumArtistKey: NormalizedBrowseTextKey(normalizedAlbumArtist),
			AlbumArtistKey:           albumArtistKey, MusicBrainzReleaseID: releaseID,
			MusicBrainzReleaseGroupID: releaseGroupID, Year: music.Year,
			DiscNumber: music.DiscNumber, TrackNumber: music.TrackNumber, Duration: music.Duration, HasCover: music.Img != "",
		}
	}

	return projection
}

// BuildMusicGenreFacets keeps the first source spelling for display while
// de-duplicating by the same normalized token used for filtering.
func BuildMusicGenreFacets(music *Music) []MusicGenreFacet {
	if music == nil {
		return []MusicGenreFacet{}
	}
	values := music.Genres
	if len(values) == 0 && strings.TrimSpace(music.Genre) != "" {
		values = StringList{music.Genre}
	}
	return buildMusicGenreFacets(music.ID, values)
}

func NormalizeGenreTokens(values StringList) StringList {
	genreTokens := TokenizeGenres(values)
	tokens := make(StringList, 0, len(genreTokens))
	seen := make(map[string]struct{}, len(genreTokens))
	for _, token := range genreTokens {
		if _, exists := seen[token.Canonical]; exists {
			continue
		}
		seen[token.Canonical] = struct{}{}
		tokens = append(tokens, token.Canonical)
	}
	return tokens
}

func buildMusicGenreFacets(musicID uint, values StringList) []MusicGenreFacet {
	tokens := TokenizeGenres(values)
	result := make([]MusicGenreFacet, 0, len(tokens))
	for _, token := range tokens {
		result = append(result, MusicGenreFacet{
			MusicID: musicID, NormalizedName: token.Normalized, DisplayName: token.Display, Position: len(result),
		})
	}
	return result
}

func NormalizeBrowseText(value string) string {
	normalized := norm.NFKC.String(value)
	normalized = strings.Join(strings.Fields(normalized), " ")
	return cases.Fold().String(normalized)
}

// NormalizedBrowseTextKey provides a bounded index key; callers still compare
// the normalized text itself so the hash never becomes an identity authority.
func NormalizedBrowseTextKey(value string) string {
	normalized := NormalizeBrowseText(value)
	sum := sha256.Sum256([]byte(normalized))
	return hex.EncodeToString(sum[:])
}

// BrowseGroupKey is opaque to clients. Stable MusicBrainz IDs remain readable
// for diagnostics; text identities do not leak potentially hostile tag values.
func BrowseGroupKey(stableID, fallback string) string {
	if stableID = normalizeBrowseStableID(stableID); stableID != "" {
		return "mbid_" + stableID
	}
	return "text_" + NormalizedBrowseTextKey(fallback)
}

func IsBrowseGroupKey(value string) bool {
	return browseGroupKeyPattern.MatchString(value)
}

func normalizeBrowseStableID(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if !browseStableIDPattern.MatchString(value) {
		return ""
	}
	return value
}
