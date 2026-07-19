// Package domain contains compatibility request shapes for legacy tag and
// MusicBee clients. Every endpoint maps these values to the canonical Music
// model; there is intentionally no second runtime MusicTag entity.
package domain

type CreateMusicTagRequest struct {
	Artist       string     `json:"artist" binding:"required"`
	Artists      StringList `json:"artists"`
	Title        string     `json:"title" binding:"required"`
	Album        string     `json:"album"`
	AlbumArtist  string     `json:"album_artist"`
	AlbumArtists StringList `json:"album_artists"`
	TrackNumber  *int       `json:"track_number"`
	TrackTotal   *int       `json:"track_total"`
	DiscNumber   *int       `json:"disc_number"`
	DiscTotal    *int       `json:"disc_total"`
	Genre        string     `json:"genre"`
	Genres       StringList `json:"genres"`
	Year         *int       `json:"year"`
	ReleaseDate  string     `json:"release_date"`
	OriginalDate string     `json:"original_release_date"`
	Duration     *int       `json:"duration"`
	Comment      string     `json:"comment"`
	ISRCs        StringList `json:"isrcs"`

	// The singular fields remain accepted as compatibility aliases only.
	MusicBrainzID             string     `json:"musicbrainz_id"`
	MusicBrainzArtistID       string     `json:"musicbrainz_artist_id"`
	MusicBrainzRecordingID    string     `json:"musicbrainz_recording_id"`
	MusicBrainzTrackID        string     `json:"musicbrainz_track_id"`
	MusicBrainzReleaseID      string     `json:"musicbrainz_release_id"`
	MusicBrainzReleaseGroupID string     `json:"musicbrainz_release_group_id"`
	MusicBrainzArtistIDs      StringList `json:"musicbrainz_artist_ids"`
	MusicBrainzAlbumArtistIDs StringList `json:"musicbrainz_album_artist_ids"`
}

type TagSearchParams struct {
	Artist                    string `form:"artist" json:"artist"`
	Title                     string `form:"title" json:"title"`
	Album                     string `form:"album" json:"album"`
	AlbumArtist               string `form:"album_artist" json:"album_artist"`
	Genre                     string `form:"genre" json:"genre"`
	Year                      *int   `form:"year" json:"year"`
	MinYear                   *int   `form:"min_year" json:"min_year"`
	MaxYear                   *int   `form:"max_year" json:"max_year"`
	Duration                  *int   `form:"duration" json:"duration"`
	MinDuration               *int   `form:"min_duration" json:"min_duration"`
	MaxDuration               *int   `form:"max_duration" json:"max_duration"`
	MusicBrainzID             string `form:"musicbrainz_id" json:"musicbrainz_id"`
	MusicBrainzRecordingID    string `form:"musicbrainz_recording_id" json:"musicbrainz_recording_id"`
	MusicBrainzTrackID        string `form:"musicbrainz_track_id" json:"musicbrainz_track_id"`
	MusicBrainzReleaseID      string `form:"musicbrainz_release_id" json:"musicbrainz_release_id"`
	MusicBrainzReleaseGroupID string `form:"musicbrainz_release_group_id" json:"musicbrainz_release_group_id"`
	Limit                     int    `form:"limit" json:"limit"`
	Offset                    int    `form:"offset" json:"offset"`
}

func (s *TagSearchParams) GetLimit() int {
	if s.Limit <= 0 || s.Limit > 100 {
		return 20
	}
	return s.Limit
}

func (s *TagSearchParams) GetOffset() int {
	if s.Offset < 0 {
		return 0
	}
	return s.Offset
}
