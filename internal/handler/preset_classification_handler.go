package handler

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/kyangconn/music-online-go/internal/repository"
	"github.com/kyangconn/music-online-go/internal/service"
)

type PresetClassificationHandler struct {
	service service.PresetClassificationService
}

func NewPresetClassificationHandler(classificationService service.PresetClassificationService) *PresetClassificationHandler {
	return &PresetClassificationHandler{service: classificationService}
}

type setManualPresetRequest struct {
	Preset string `json:"preset" binding:"required"`
}

func (h *PresetClassificationHandler) Reclassify(c *gin.Context) {
	musicID, ok := classificationMusicID(c)
	if !ok {
		return
	}
	classification, err := h.service.Reclassify(c.Request.Context(), musicID)
	if err != nil {
		handlePresetClassificationError(c, err)
		return
	}
	Success(c, classification)
}

func (h *PresetClassificationHandler) SetManualPreset(c *gin.Context) {
	musicID, ok := classificationMusicID(c)
	if !ok {
		return
	}
	var req setManualPresetRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, "Invalid preset override")
		return
	}
	classification, err := h.service.SetManualPreset(c.Request.Context(), musicID, c.GetUint("userID"), req.Preset)
	if err != nil {
		handlePresetClassificationError(c, err)
		return
	}
	Success(c, classification)
}

func (h *PresetClassificationHandler) ClearManualPreset(c *gin.Context) {
	musicID, ok := classificationMusicID(c)
	if !ok {
		return
	}
	classification, err := h.service.ClearManualPreset(c.Request.Context(), musicID)
	if err != nil {
		handlePresetClassificationError(c, err)
		return
	}
	Success(c, classification)
}

func classificationMusicID(c *gin.Context) (uint, bool) {
	value, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil || value == 0 {
		BadRequest(c, "Invalid music ID")
		return 0, false
	}
	return uint(value), true
}

func handlePresetClassificationError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrInvalidPreset):
		BadRequest(c, "Invalid preset")
	case errors.Is(err, repository.ErrMusicNotFound), errors.Is(err, repository.ErrPresetClassificationNotFound):
		NotFound(c, "Music not found")
	case errors.Is(err, service.ErrClassificationDisabled):
		Error(c, http.StatusServiceUnavailable, "Preset classification is disabled")
	default:
		InternalServerError(c, "Preset classification operation failed")
	}
}
