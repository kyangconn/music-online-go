// Package handler exposes compatibility metadata endpoints backed by Music,
// the canonical track model. The legacy music_tags table is not used at
// runtime after migration 3.
package handler

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/kyangconn/music-online-go/internal/domain"
	"github.com/kyangconn/music-online-go/internal/repository"
	"github.com/kyangconn/music-online-go/internal/service"
)

type MusicTagHandler struct {
	service service.MusicService
}

type MatchTagResponse struct {
	IsMatched  bool                    `json:"is_matched"`
	MatchType  string                  `json:"match_type"`
	Tag        *domain.MusicResponse   `json:"tag,omitempty"`
	Candidates []*domain.MusicResponse `json:"candidates"`
}

func NewMusicTagHandler(svc service.MusicService) *MusicTagHandler {
	return &MusicTagHandler{service: svc}
}

func (h *MusicTagHandler) SearchMusicTags(c *gin.Context) {
	var params domain.TagSearchParams
	if err := c.ShouldBindJSON(&params); err != nil {
		BadRequest(c, err.Error())
		return
	}
	musics, total, err := h.search(c, &params)
	if err != nil {
		InternalServerError(c, err.Error())
		return
	}
	Success(c, gin.H{
		"tags":   musics,
		"total":  total,
		"limit":  params.GetLimit(),
		"offset": params.GetOffset(),
	})
}

func (h *MusicTagHandler) MatchTags(c *gin.Context) {
	var req domain.CreateMusicTagRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, err.Error())
		return
	}
	candidates, stable, err := h.service.FindMetadataCandidates(c.Request.Context(), tagMetadata(&req), optionalUserID(c))
	if err != nil {
		if errors.Is(err, service.ErrInvalidMusicMetadata) {
			BadRequest(c, err.Error())
			return
		}
		InternalServerError(c, err.Error())
		return
	}
	response := MatchTagResponse{
		IsMatched:  stable && len(candidates) > 0,
		MatchType:  "none",
		Candidates: candidates,
	}
	if len(candidates) > 0 {
		if stable {
			response.MatchType = "stable_id"
			response.Tag = candidates[0]
		} else {
			response.MatchType = "text_candidate"
		}
	}
	Success(c, response)
}

func (h *MusicTagHandler) SearchTracks(c *gin.Context) {
	var params domain.TagSearchParams
	if err := c.ShouldBindJSON(&params); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	musics, total, err := h.search(c, &params)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"tracks": musics,
		"total":  total,
		"count":  len(musics),
	})
}

func (h *MusicTagHandler) SubmitTrack(c *gin.Context) {
	var req domain.CreateMusicTagRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	music, err := h.service.Create(c.Request.Context(), c.GetUint("userID"), tagCreateRequest(&req))
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, service.ErrInvalidMusicMetadata) {
			status = http.StatusBadRequest
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, music)
}

func (h *MusicTagHandler) LookupByMBID(c *gin.Context) {
	mbid := c.Query("musicbrainz_recording_id")
	if mbid == "" {
		mbid = c.Query("musicbrainz_id")
	}
	if mbid == "" {
		BadRequest(c, "musicbrainz_recording_id parameter required")
		return
	}
	music, err := h.service.FindByMusicBrainzRecordingID(c.Request.Context(), mbid, optionalUserID(c))
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidMusicMetadata):
			BadRequest(c, err.Error())
		case errors.Is(err, repository.ErrMusicNotFound):
			NotFound(c, "Music not found")
		default:
			InternalServerError(c, err.Error())
		}
		return
	}
	Success(c, music)
}

func (h *MusicTagHandler) search(c *gin.Context, params *domain.TagSearchParams) ([]*domain.MusicResponse, int64, error) {
	limit := params.GetLimit()
	offset := params.GetOffset()
	recordingID := params.MusicBrainzRecordingID
	if recordingID == "" {
		recordingID = params.MusicBrainzID
	}
	recordingID = strings.ToLower(strings.TrimSpace(recordingID))
	return h.service.Search(c.Request.Context(), &domain.MusicSearchParams{
		Title:          strings.TrimSpace(params.Title),
		Artist:         strings.TrimSpace(params.Artist),
		Album:          strings.TrimSpace(params.Album),
		AlbumArtist:    strings.TrimSpace(params.AlbumArtist),
		Genre:          strings.TrimSpace(params.Genre),
		Year:           params.Year,
		MinYear:        params.MinYear,
		MaxYear:        params.MaxYear,
		Duration:       params.Duration,
		MinDuration:    params.MinDuration,
		MaxDuration:    params.MaxDuration,
		RecordingID:    recordingID,
		TrackID:        strings.ToLower(strings.TrimSpace(params.MusicBrainzTrackID)),
		ReleaseID:      strings.ToLower(strings.TrimSpace(params.MusicBrainzReleaseID)),
		ReleaseGroupID: strings.ToLower(strings.TrimSpace(params.MusicBrainzReleaseGroupID)),
		Page:           1,
		PageSize:       limit,
		Offset:         &offset,
	}, optionalUserID(c))
}

func tagCreateRequest(req *domain.CreateMusicTagRequest) *domain.CreateMusicRequest {
	metadata := tagMetadata(req)
	return &domain.CreateMusicRequest{
		Title:                     metadata.Title,
		Artist:                    metadata.Artist,
		Artists:                   metadata.Artists,
		Album:                     metadata.Album,
		AlbumArtist:               metadata.AlbumArtist,
		AlbumArtists:              metadata.AlbumArtists,
		Year:                      metadata.Year,
		TrackNumber:               metadata.TrackNumber,
		TrackTotal:                metadata.TrackTotal,
		DiscNumber:                metadata.DiscNumber,
		DiscTotal:                 metadata.DiscTotal,
		ReleaseDate:               metadata.ReleaseDate,
		OriginalReleaseDate:       metadata.OriginalReleaseDate,
		Genre:                     metadata.Genre,
		Genres:                    metadata.Genres,
		Comment:                   metadata.Comment,
		ISRCs:                     metadata.ISRCs,
		Duration:                  metadata.Duration,
		MusicBrainzRecordingID:    metadata.MusicBrainzRecordingID,
		MusicBrainzTrackID:        metadata.MusicBrainzTrackID,
		MusicBrainzReleaseID:      metadata.MusicBrainzReleaseID,
		MusicBrainzReleaseGroupID: metadata.MusicBrainzReleaseGroupID,
		MusicBrainzArtistIDs:      metadata.MusicBrainzArtistIDs,
		MusicBrainzAlbumArtistIDs: metadata.MusicBrainzAlbumArtistIDs,
		Type:                      domain.MusicTypeSingle,
	}
}

func tagMetadata(req *domain.CreateMusicTagRequest) domain.MusicMetadata {
	recordingID := req.MusicBrainzRecordingID
	if recordingID == "" {
		recordingID = req.MusicBrainzID
	}
	artistIDs := req.MusicBrainzArtistIDs
	if len(artistIDs) == 0 && req.MusicBrainzArtistID != "" {
		artistIDs = domain.StringList{req.MusicBrainzArtistID}
	}
	return domain.MusicMetadata{
		Title:                     req.Title,
		Artist:                    req.Artist,
		Artists:                   req.Artists,
		Album:                     req.Album,
		AlbumArtist:               req.AlbumArtist,
		AlbumArtists:              req.AlbumArtists,
		Year:                      intValue(req.Year),
		TrackNumber:               intValue(req.TrackNumber),
		TrackTotal:                intValue(req.TrackTotal),
		DiscNumber:                intValue(req.DiscNumber),
		DiscTotal:                 intValue(req.DiscTotal),
		ReleaseDate:               req.ReleaseDate,
		OriginalReleaseDate:       req.OriginalDate,
		Genre:                     req.Genre,
		Genres:                    req.Genres,
		Comment:                   req.Comment,
		ISRCs:                     req.ISRCs,
		Duration:                  intValue(req.Duration),
		MusicBrainzRecordingID:    recordingID,
		MusicBrainzTrackID:        req.MusicBrainzTrackID,
		MusicBrainzReleaseID:      req.MusicBrainzReleaseID,
		MusicBrainzReleaseGroupID: req.MusicBrainzReleaseGroupID,
		MusicBrainzArtistIDs:      artistIDs,
		MusicBrainzAlbumArtistIDs: req.MusicBrainzAlbumArtistIDs,
	}
}

func optionalUserID(c *gin.Context) *uint {
	if _, exists := c.Get("userID"); !exists {
		return nil
	}
	value := c.GetUint("userID")
	return &value
}

func intValue(value *int) int {
	if value == nil {
		return 0
	}
	return *value
}
