package handler

import (
	"errors"
	"io"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/kyangconn/music-online-go/internal/domain"
	"github.com/kyangconn/music-online-go/internal/repository"
	"github.com/kyangconn/music-online-go/internal/service"
)

type MusicAnalysisHandler struct {
	service service.MusicAnalysisService
}

func NewMusicAnalysisHandler(analysisService service.MusicAnalysisService) *MusicAnalysisHandler {
	return &MusicAnalysisHandler{service: analysisService}
}

func (handler *MusicAnalysisHandler) ScheduleMusic(c *gin.Context) {
	musicID, ok := classificationMusicID(c)
	if !ok {
		return
	}
	request := domain.AnalysisEnqueueRequest{IncludeAudio: true}
	if err := bindOptionalAnalysisJSON(c, &request); err != nil {
		BadRequest(c, "Invalid music analysis request")
		return
	}
	response, err := handler.service.ScheduleMusic(c.Request.Context(), musicID, c.GetUint("userID"), request)
	if err != nil {
		handleMusicAnalysisError(c, err)
		return
	}
	accepted(c, response)
}

func (handler *MusicAnalysisHandler) Backfill(c *gin.Context) {
	var request domain.AnalysisBackfillRequest
	if err := bindOptionalAnalysisJSON(c, &request); err != nil {
		BadRequest(c, "Invalid music analysis backfill request")
		return
	}
	response, err := handler.service.Backfill(c.Request.Context(), c.GetUint("userID"), request)
	if err != nil {
		handleMusicAnalysisError(c, err)
		return
	}
	accepted(c, response)
}

func (handler *MusicAnalysisHandler) ListJobs(c *gin.Context) {
	params := domain.AnalysisJobListParams{Kind: c.Query("kind"), Status: c.Query("status")}
	if params.Kind != "" && !domain.IsAnalysisJobKind(params.Kind) {
		BadRequest(c, "Invalid music analysis job kind")
		return
	}
	if params.Status != "" && !domain.IsAnalysisJobStatus(params.Status) {
		BadRequest(c, "Invalid music analysis job status")
		return
	}
	if value := c.Query("music_id"); value != "" {
		parsed, err := strconv.ParseUint(value, 10, 32)
		if err != nil || parsed == 0 {
			BadRequest(c, "Invalid music ID")
			return
		}
		musicID := uint(parsed)
		params.MusicID = &musicID
	}
	params.Page, params.PageSize = parsePagination(c, 20)
	jobs, total, err := handler.service.ListJobs(c.Request.Context(), params)
	if err != nil {
		handleMusicAnalysisError(c, err)
		return
	}
	Success(c, gin.H{"items": jobs, "total": total, "page": params.Page, "size": params.PageSize})
}

func (handler *MusicAnalysisHandler) GetJob(c *gin.Context) {
	id, ok := analysisJobID(c)
	if !ok {
		return
	}
	job, err := handler.service.GetJob(c.Request.Context(), id)
	if err != nil {
		handleMusicAnalysisError(c, err)
		return
	}
	Success(c, job)
}

func (handler *MusicAnalysisHandler) CancelJob(c *gin.Context) {
	id, ok := analysisJobID(c)
	if !ok {
		return
	}
	job, err := handler.service.CancelJob(c.Request.Context(), id)
	if err != nil {
		handleMusicAnalysisError(c, err)
		return
	}
	Success(c, job)
}

func (handler *MusicAnalysisHandler) Metrics(c *gin.Context) {
	metrics, err := handler.service.Metrics(c.Request.Context())
	if err != nil {
		handleMusicAnalysisError(c, err)
		return
	}
	Success(c, metrics)
}

func bindOptionalAnalysisJSON(c *gin.Context, target any) error {
	if c.Request.Body == nil || c.Request.ContentLength == 0 {
		return nil
	}
	if err := c.ShouldBindJSON(target); errors.Is(err, io.EOF) {
		return nil
	} else {
		return err
	}
}

func analysisJobID(c *gin.Context) (uint, bool) {
	value, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil || value == 0 {
		BadRequest(c, "Invalid music analysis job ID")
		return 0, false
	}
	return uint(value), true
}

func accepted(c *gin.Context, data any) {
	c.JSON(http.StatusAccepted, Response{Code: http.StatusAccepted, Message: "accepted", Data: data})
}

func handleMusicAnalysisError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, repository.ErrMusicNotFound):
		NotFound(c, "Music not found")
	case errors.Is(err, repository.ErrAnalysisJobNotFound):
		NotFound(c, "Music analysis job not found")
	case errors.Is(err, repository.ErrAnalysisQueueFull):
		Error(c, http.StatusTooManyRequests, "Music analysis queue is full")
	case errors.Is(err, repository.ErrAnalysisJobActive):
		Error(c, http.StatusConflict, "Music analysis is already active")
	case errors.Is(err, service.ErrMusicAnalysisDisabled):
		Error(c, http.StatusServiceUnavailable, "Music analysis is disabled")
	case errors.Is(err, service.ErrAudioAnalyzerDisabled):
		Error(c, http.StatusServiceUnavailable, "Audio analyzer is disabled")
	case errors.Is(err, service.ErrAnalysisSourceMissing):
		Error(c, http.StatusUnprocessableEntity, "Music has no analyzable audio source")
	default:
		InternalServerError(c, "Music analysis operation failed")
	}
}
