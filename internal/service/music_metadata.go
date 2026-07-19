package service

import (
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/kyangconn/music-online-go/internal/domain"
)

var (
	ErrInvalidMusicMetadata = errors.New("invalid music metadata")
	metadataListSeparator   = regexp.MustCompile(`[;,/]+`)
	metadataWhitespace      = regexp.MustCompile(`\s+`)
	mbidPattern             = regexp.MustCompile(`(?i)^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
	isrcPattern             = regexp.MustCompile(`^[A-Z]{2}[A-Z0-9]{3}[0-9]{7}$`)
)

const (
	maxCanonicalTitleBytes     = 255
	maxCanonicalArtistBytes    = 255
	maxCanonicalAlbumBytes     = 255
	maxCanonicalGenreBytes     = 500
	maxCanonicalCommentBytes   = 16 * 1024
	maxCanonicalMetadataValues = 256
)

func applyCreateMusicMetadata(music *domain.Music, req *domain.CreateMusicRequest) error {
	music.Title = req.Title
	music.Artist = req.Artist
	music.Artists = copyStringList(req.Artists)
	music.Album = req.Album
	music.AlbumArtist = req.AlbumArtist
	music.AlbumArtists = copyStringList(req.AlbumArtists)
	music.Year = req.Year
	music.TrackNumber = req.TrackNumber
	music.TrackTotal = req.TrackTotal
	music.DiscNumber = req.DiscNumber
	music.DiscTotal = req.DiscTotal
	music.ReleaseDate = req.ReleaseDate
	music.OriginalReleaseDate = req.OriginalReleaseDate
	music.Genre = req.Genre
	music.Genres = copyStringList(req.Genres)
	music.Comment = req.Comment
	music.ISRCs = copyStringList(req.ISRCs)
	music.Duration = req.Duration
	music.MusicBrainzRecordingID = req.MusicBrainzRecordingID
	music.MusicBrainzTrackID = req.MusicBrainzTrackID
	music.MusicBrainzReleaseID = req.MusicBrainzReleaseID
	music.MusicBrainzReleaseGroupID = req.MusicBrainzReleaseGroupID
	music.MusicBrainzArtistIDs = copyStringList(req.MusicBrainzArtistIDs)
	music.MusicBrainzAlbumArtistIDs = copyStringList(req.MusicBrainzAlbumArtistIDs)
	music.MetadataRevision = 1
	return normalizeMusicMetadata(music)
}

func applyUpdateMusicMetadata(music *domain.Music, req *domain.UpdateMusicRequest) (bool, error) {
	requested := metadataUpdateRequested(req)
	before := musicMetadataFromMusic(music)
	if req.Title != nil {
		music.Title = *req.Title
	}
	if req.Artist != nil {
		music.Artist = *req.Artist
	}
	if req.Artists != nil {
		music.Artists = copyStringList(*req.Artists)
	}
	if req.Album != nil {
		music.Album = *req.Album
	}
	if req.AlbumArtist != nil {
		music.AlbumArtist = *req.AlbumArtist
	}
	if req.AlbumArtists != nil {
		music.AlbumArtists = copyStringList(*req.AlbumArtists)
	}
	if req.Year != nil {
		music.Year = *req.Year
	}
	if req.TrackNumber != nil {
		music.TrackNumber = *req.TrackNumber
	}
	if req.TrackTotal != nil {
		music.TrackTotal = *req.TrackTotal
	}
	if req.DiscNumber != nil {
		music.DiscNumber = *req.DiscNumber
	}
	if req.DiscTotal != nil {
		music.DiscTotal = *req.DiscTotal
	}
	if req.ReleaseDate != nil {
		music.ReleaseDate = *req.ReleaseDate
	}
	if req.OriginalReleaseDate != nil {
		music.OriginalReleaseDate = *req.OriginalReleaseDate
	}
	if req.Genre != nil {
		music.Genre = *req.Genre
		if req.Genres == nil {
			music.Genres = nil
		}
	}
	if req.Genres != nil {
		music.Genres = copyStringList(*req.Genres)
	}
	if req.Comment != nil {
		music.Comment = *req.Comment
	}
	if req.ISRCs != nil {
		music.ISRCs = copyStringList(*req.ISRCs)
	}
	if req.Duration != nil {
		music.Duration = *req.Duration
	}
	if req.MusicBrainzRecordingID != nil {
		music.MusicBrainzRecordingID = *req.MusicBrainzRecordingID
	}
	if req.MusicBrainzTrackID != nil {
		music.MusicBrainzTrackID = *req.MusicBrainzTrackID
	}
	if req.MusicBrainzReleaseID != nil {
		music.MusicBrainzReleaseID = *req.MusicBrainzReleaseID
	}
	if req.MusicBrainzReleaseGroupID != nil {
		music.MusicBrainzReleaseGroupID = *req.MusicBrainzReleaseGroupID
	}
	if req.MusicBrainzArtistIDs != nil {
		music.MusicBrainzArtistIDs = copyStringList(*req.MusicBrainzArtistIDs)
	}
	if req.MusicBrainzAlbumArtistIDs != nil {
		music.MusicBrainzAlbumArtistIDs = copyStringList(*req.MusicBrainzAlbumArtistIDs)
	}

	if !requested {
		return false, nil
	}
	if err := normalizeMusicMetadata(music); err != nil {
		return false, err
	}
	if musicMetadataEqual(before, musicMetadataFromMusic(music)) {
		return false, nil
	}
	if music.MetadataRevision == 0 {
		music.MetadataRevision = 1
	} else {
		music.MetadataRevision++
	}
	return true, nil
}

func normalizeMusicMetadata(music *domain.Music) error {
	music.Title = strings.TrimSpace(music.Title)
	music.Artist = strings.TrimSpace(music.Artist)
	if music.Title == "" || music.Artist == "" {
		return fmt.Errorf("%w: title and artist are required", ErrInvalidMusicMetadata)
	}
	if err := validateCanonicalText("title", music.Title, maxCanonicalTitleBytes); err != nil {
		return err
	}
	if err := validateCanonicalText("artist", music.Artist, maxCanonicalArtistBytes); err != nil {
		return err
	}

	music.Artists = normalizeDisplayValues(music.Artists, false)
	if len(music.Artists) == 0 {
		music.Artists = domain.StringList{music.Artist}
	}
	if err := validateCanonicalValues("artists", music.Artists, maxCanonicalArtistBytes); err != nil {
		return err
	}
	music.Album = strings.TrimSpace(music.Album)
	if err := validateCanonicalText("album", music.Album, maxCanonicalAlbumBytes); err != nil {
		return err
	}
	music.AlbumArtist = strings.TrimSpace(music.AlbumArtist)
	music.AlbumArtists = normalizeDisplayValues(music.AlbumArtists, false)
	if err := validateCanonicalValues("album_artists", music.AlbumArtists, maxCanonicalArtistBytes); err != nil {
		return err
	}
	if music.AlbumArtist == "" && len(music.AlbumArtists) > 0 {
		music.AlbumArtist = joinMetadataDisplayValues(music.AlbumArtists, maxCanonicalArtistBytes)
	}
	if err := validateCanonicalText("album_artist", music.AlbumArtist, maxCanonicalArtistBytes); err != nil {
		return err
	}
	if len(music.AlbumArtists) == 0 && music.AlbumArtist != "" {
		music.AlbumArtists = domain.StringList{music.AlbumArtist}
	}
	if music.Year != 0 && (music.Year < 1000 || music.Year > 9999) {
		return fmt.Errorf("%w: year must be zero or between 1000 and 9999", ErrInvalidMusicMetadata)
	}
	if music.Duration < 0 {
		return fmt.Errorf("%w: duration cannot be negative", ErrInvalidMusicMetadata)
	}

	if err := validateTrackAndDiscTotals(music); err != nil {
		return err
	}
	music.ReleaseDate = strings.TrimSpace(music.ReleaseDate)
	music.OriginalReleaseDate = strings.TrimSpace(music.OriginalReleaseDate)
	if err := validatePartialDate("release_date", music.ReleaseDate); err != nil {
		return err
	}
	if err := validatePartialDate("original_release_date", music.OriginalReleaseDate); err != nil {
		return err
	}
	if music.Year == 0 && len(music.ReleaseDate) >= 4 {
		music.Year, _ = strconv.Atoi(music.ReleaseDate[:4])
	}

	if len(music.Genres) == 0 && strings.TrimSpace(music.Genre) != "" {
		music.Genres = domain.StringList{music.Genre}
	}
	// Genres retain the source tag values for display and round trips. The
	// derived GenreTokens field is the only representation that splits common
	// delimiters and folds case for matching and future M5 rules.
	music.Genres = normalizeDisplayValues(music.Genres, false)
	if err := validateCanonicalValues("genres", music.Genres, maxCanonicalGenreBytes); err != nil {
		return err
	}
	music.Genre = joinMetadataDisplayValues(music.Genres, maxCanonicalGenreBytes)
	music.GenreTokens = normalizeGenreTokens(music.Genres)
	music.Comment = strings.TrimSpace(music.Comment)
	if err := validateCanonicalText("comment", music.Comment, maxCanonicalCommentBytes); err != nil {
		return err
	}
	var err error
	if music.ISRCs, err = normalizeISRCs(music.ISRCs); err != nil {
		return err
	}

	if music.MusicBrainzRecordingID, err = normalizeMBID("musicbrainz_recording_id", music.MusicBrainzRecordingID); err != nil {
		return err
	}
	if music.MusicBrainzTrackID, err = normalizeMBID("musicbrainz_track_id", music.MusicBrainzTrackID); err != nil {
		return err
	}
	if music.MusicBrainzReleaseID, err = normalizeMBID("musicbrainz_release_id", music.MusicBrainzReleaseID); err != nil {
		return err
	}
	if music.MusicBrainzReleaseGroupID, err = normalizeMBID("musicbrainz_release_group_id", music.MusicBrainzReleaseGroupID); err != nil {
		return err
	}
	if music.MusicBrainzArtistIDs, err = normalizeMBIDs("musicbrainz_artist_ids", music.MusicBrainzArtistIDs); err != nil {
		return err
	}
	if music.MusicBrainzAlbumArtistIDs, err = normalizeMBIDs("musicbrainz_album_artist_ids", music.MusicBrainzAlbumArtistIDs); err != nil {
		return err
	}
	return nil
}

func validateCanonicalText(field, value string, maxBytes int) error {
	if strings.ContainsRune(value, '\x00') {
		return fmt.Errorf("%w: %s cannot contain NUL bytes", ErrInvalidMusicMetadata, field)
	}
	if len(value) > maxBytes {
		return fmt.Errorf("%w: %s exceeds %d UTF-8 bytes", ErrInvalidMusicMetadata, field, maxBytes)
	}
	return nil
}

func validateCanonicalValues(field string, values domain.StringList, maxValueBytes int) error {
	if len(values) > maxCanonicalMetadataValues {
		return fmt.Errorf("%w: %s contains more than %d values", ErrInvalidMusicMetadata, field, maxCanonicalMetadataValues)
	}
	for index, value := range values {
		if err := validateCanonicalText(fmt.Sprintf("%s[%d]", field, index), value, maxValueBytes); err != nil {
			return err
		}
	}
	return nil
}

// Compatibility scalar columns stay bounded while the complete, lossless
// multi-value representation remains available in the corresponding list.
func joinMetadataDisplayValues(values domain.StringList, maxBytes int) string {
	var builder strings.Builder
	for _, value := range values {
		separator := ""
		if builder.Len() > 0 {
			separator = "; "
		}
		if builder.Len()+len(separator)+len(value) > maxBytes {
			break
		}
		builder.WriteString(separator)
		builder.WriteString(value)
	}
	return builder.String()
}

func metadataUpdateRequested(req *domain.UpdateMusicRequest) bool {
	return req.Title != nil || req.Artist != nil || req.Artists != nil || req.Album != nil ||
		req.AlbumArtist != nil || req.AlbumArtists != nil || req.Year != nil || req.TrackNumber != nil ||
		req.TrackTotal != nil || req.DiscNumber != nil || req.DiscTotal != nil || req.ReleaseDate != nil ||
		req.OriginalReleaseDate != nil || req.Genre != nil || req.Genres != nil || req.Comment != nil ||
		req.ISRCs != nil || req.Duration != nil || req.MusicBrainzRecordingID != nil ||
		req.MusicBrainzTrackID != nil || req.MusicBrainzReleaseID != nil ||
		req.MusicBrainzReleaseGroupID != nil || req.MusicBrainzArtistIDs != nil ||
		req.MusicBrainzAlbumArtistIDs != nil
}

func normalizeDisplayValues(values domain.StringList, split bool) domain.StringList {
	normalized := make(domain.StringList, 0, len(values))
	seen := make(map[string]struct{})
	for _, value := range values {
		parts := []string{value}
		if split {
			parts = metadataListSeparator.Split(value, -1)
		}
		for _, part := range parts {
			display := metadataWhitespace.ReplaceAllString(strings.TrimSpace(part), " ")
			if display == "" {
				continue
			}
			key := strings.ToLower(display)
			if _, exists := seen[key]; exists {
				continue
			}
			seen[key] = struct{}{}
			normalized = append(normalized, display)
		}
	}
	return normalized
}

func normalizeGenreTokens(genres domain.StringList) domain.StringList {
	tokens := make(domain.StringList, 0, len(genres))
	seen := make(map[string]struct{})
	for _, genre := range genres {
		for _, value := range metadataListSeparator.Split(genre, -1) {
			token := strings.ToLower(metadataWhitespace.ReplaceAllString(strings.TrimSpace(value), " "))
			if token == "" {
				continue
			}
			if _, exists := seen[token]; exists {
				continue
			}
			seen[token] = struct{}{}
			tokens = append(tokens, token)
			if len(tokens) >= maxCanonicalMetadataValues {
				return tokens
			}
		}
	}
	return tokens
}

func normalizeISRCs(values domain.StringList) (domain.StringList, error) {
	normalized := make(domain.StringList, 0, len(values))
	seen := make(map[string]struct{})
	for _, value := range normalizeDisplayValues(values, true) {
		isrc := strings.ToUpper(strings.NewReplacer("-", "", " ", "").Replace(value))
		if !isrcPattern.MatchString(isrc) {
			return nil, fmt.Errorf("%w: isrcs must contain valid 12-character ISRC values", ErrInvalidMusicMetadata)
		}
		if _, exists := seen[isrc]; exists {
			continue
		}
		seen[isrc] = struct{}{}
		normalized = append(normalized, isrc)
		if len(normalized) > maxCanonicalMetadataValues {
			return nil, fmt.Errorf("%w: isrcs contains more than %d values", ErrInvalidMusicMetadata, maxCanonicalMetadataValues)
		}
	}
	return normalized, nil
}

func normalizeMBID(field, value string) (string, error) {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return "", nil
	}
	if !mbidPattern.MatchString(value) {
		return "", fmt.Errorf("%w: %s must be a MusicBrainz UUID", ErrInvalidMusicMetadata, field)
	}
	return value, nil
}

func normalizeMBIDs(field string, values domain.StringList) (domain.StringList, error) {
	normalized := make(domain.StringList, 0, len(values))
	seen := make(map[string]struct{})
	for _, value := range values {
		mbid, err := normalizeMBID(field, value)
		if err != nil {
			return nil, err
		}
		if mbid == "" {
			continue
		}
		if _, exists := seen[mbid]; exists {
			continue
		}
		seen[mbid] = struct{}{}
		normalized = append(normalized, mbid)
		if len(normalized) > maxCanonicalMetadataValues {
			return nil, fmt.Errorf("%w: %s contains more than %d values", ErrInvalidMusicMetadata, field, maxCanonicalMetadataValues)
		}
	}
	return normalized, nil
}

func validateTrackAndDiscTotals(music *domain.Music) error {
	if music.TrackNumber < 0 || music.TrackTotal < 0 || music.DiscNumber < 0 || music.DiscTotal < 0 {
		return fmt.Errorf("%w: track and disc values cannot be negative", ErrInvalidMusicMetadata)
	}
	if music.TrackTotal > 0 && music.TrackNumber > music.TrackTotal {
		return fmt.Errorf("%w: track_number cannot exceed track_total", ErrInvalidMusicMetadata)
	}
	if music.DiscTotal > 0 && music.DiscNumber > music.DiscTotal {
		return fmt.Errorf("%w: disc_number cannot exceed disc_total", ErrInvalidMusicMetadata)
	}
	return nil
}

func validatePartialDate(field, value string) error {
	if value == "" {
		return nil
	}
	formats := map[int]string{4: "2006", 7: "2006-01", 10: "2006-01-02"}
	format, ok := formats[len(value)]
	if !ok {
		return fmt.Errorf("%w: %s must use YYYY, YYYY-MM, or YYYY-MM-DD", ErrInvalidMusicMetadata, field)
	}
	if _, err := time.Parse(format, value); err != nil {
		return fmt.Errorf("%w: invalid %s", ErrInvalidMusicMetadata, field)
	}
	return nil
}

func copyStringList(values domain.StringList) domain.StringList {
	if len(values) == 0 {
		return domain.StringList{}
	}
	return append(domain.StringList{}, values...)
}

func musicMetadataFromMusic(music *domain.Music) domain.MusicMetadata {
	return domain.MusicMetadata{
		Title:                     music.Title,
		Artist:                    music.Artist,
		Artists:                   copyStringList(music.Artists),
		Album:                     music.Album,
		AlbumArtist:               music.AlbumArtist,
		AlbumArtists:              copyStringList(music.AlbumArtists),
		Year:                      music.Year,
		TrackNumber:               music.TrackNumber,
		TrackTotal:                music.TrackTotal,
		DiscNumber:                music.DiscNumber,
		DiscTotal:                 music.DiscTotal,
		ReleaseDate:               music.ReleaseDate,
		OriginalReleaseDate:       music.OriginalReleaseDate,
		Genre:                     music.Genre,
		Genres:                    copyStringList(music.Genres),
		Comment:                   music.Comment,
		ISRCs:                     copyStringList(music.ISRCs),
		Duration:                  music.Duration,
		MusicBrainzRecordingID:    music.MusicBrainzRecordingID,
		MusicBrainzTrackID:        music.MusicBrainzTrackID,
		MusicBrainzReleaseID:      music.MusicBrainzReleaseID,
		MusicBrainzReleaseGroupID: music.MusicBrainzReleaseGroupID,
		MusicBrainzArtistIDs:      copyStringList(music.MusicBrainzArtistIDs),
		MusicBrainzAlbumArtistIDs: copyStringList(music.MusicBrainzAlbumArtistIDs),
	}
}

func musicMetadataEqual(left, right domain.MusicMetadata) bool {
	return left.Title == right.Title && left.Artist == right.Artist &&
		slices.Equal(left.Artists, right.Artists) && left.Album == right.Album &&
		left.AlbumArtist == right.AlbumArtist && slices.Equal(left.AlbumArtists, right.AlbumArtists) &&
		left.Year == right.Year && left.TrackNumber == right.TrackNumber && left.TrackTotal == right.TrackTotal &&
		left.DiscNumber == right.DiscNumber && left.DiscTotal == right.DiscTotal &&
		left.ReleaseDate == right.ReleaseDate && left.OriginalReleaseDate == right.OriginalReleaseDate &&
		left.Genre == right.Genre && slices.Equal(left.Genres, right.Genres) && left.Comment == right.Comment &&
		slices.Equal(left.ISRCs, right.ISRCs) && left.Duration == right.Duration &&
		left.MusicBrainzRecordingID == right.MusicBrainzRecordingID &&
		left.MusicBrainzTrackID == right.MusicBrainzTrackID &&
		left.MusicBrainzReleaseID == right.MusicBrainzReleaseID &&
		left.MusicBrainzReleaseGroupID == right.MusicBrainzReleaseGroupID &&
		slices.Equal(left.MusicBrainzArtistIDs, right.MusicBrainzArtistIDs) &&
		slices.Equal(left.MusicBrainzAlbumArtistIDs, right.MusicBrainzAlbumArtistIDs)
}
