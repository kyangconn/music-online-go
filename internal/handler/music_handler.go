// Package handler music_handler.go - 音乐处理器
// 处理音乐相关的 HTTP 请求：创建、查询、更新、删除、流式播放、上传文件
package handler

import (
	"errors"
	"fmt"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/kyangconn/music-online-go/internal/domain"
	pklog "github.com/kyangconn/music-online-go/internal/pkg/log"
	"github.com/kyangconn/music-online-go/internal/repository"
	"github.com/kyangconn/music-online-go/internal/service"
)

// MusicHandler handles HTTP requests related to music operations.
type MusicHandler struct {
	service service.MusicService
}

func NewMusicHandler(musicService service.MusicService) *MusicHandler {
	return &MusicHandler{service: musicService}
}

func (h *MusicHandler) UploadPolicy(c *gin.Context) {
	Success(c, service.CurrentUploadPolicy())
}

// Create godoc
// @Summary 创建音乐
// @Tags musics
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body domain.CreateMusicRequest true "音乐信息"
// @Success 201 {object} Response "创建成功"
// @Router /api/v1/musics [post]
func (h *MusicHandler) Create(c *gin.Context) {
	var req domain.CreateMusicRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, "Invalid request parameters")
		return
	}

	userID := c.GetUint("userID")
	music, err := h.service.Create(c.Request.Context(), userID, &req)
	if err != nil {
		InternalServerError(c, "Failed to create music")
		return
	}

	Created(c, music)
}

// UploadFile godoc
// @Summary 上传音频/封面文件
// @Tags musics
// @Accept multipart/form-data
// @Produce json
// @Security BearerAuth
// @Param id path int true "音乐ID"
// @Param file formData file false "音频文件"
// @Param cover formData file false "封面图片"
// @Success 200 {object} Response "上传成功"
// @Router /api/v1/musics/{id}/upload [post]
func (h *MusicHandler) UploadFile(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		BadRequest(c, "Invalid music ID")
		return
	}

	audioFile, _ := c.FormFile("file")
	coverFile, _ := c.FormFile("cover")

	if audioFile == nil && coverFile == nil {
		BadRequest(c, "At least one of 'file' or 'cover' is required")
		return
	}

	music, err := h.service.UploadFiles(c.Request.Context(), c.GetUint("userID"), c.GetString("role"), uint(id), audioFile, coverFile)
	if err != nil {
		if errors.Is(err, service.ErrForbidden) {
			Forbidden(c, "You can only upload files for your own music")
			return
		}
		if errors.Is(err, repository.ErrMusicNotFound) {
			NotFound(c, "Music not found")
			return
		}
		if errors.Is(err, service.ErrInvalidMediaFile) {
			BadRequest(c, err.Error())
			return
		}
		InternalServerError(c, "Failed to upload files")
		return
	}

	Success(c, music)
}

// GetByID godoc
// @Summary 获取音乐详情
// @Tags musics
// @Accept json
// @Produce json
// @Param id path int true "音乐ID"
// @Success 200 {object} Response "获取成功"
// @Router /api/v1/musics/{id} [get]
func (h *MusicHandler) GetByID(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		BadRequest(c, "Invalid music ID")
		return
	}

	currentUserID := h.getUserID(c)
	music, err := h.service.GetByID(c.Request.Context(), uint(id), currentUserID)
	if err != nil {
		NotFound(c, "Music not found")
		return
	}

	Success(c, music)
}

// Search godoc
// @Summary 搜索音乐
// @Tags musics
// @Accept json
// @Produce json
// @Param q query string false "搜索关键词"
// @Param page query int false "页码"
// @Param page_size query int false "每页数量"
// @Success 200 {object} Response "搜索成功"
// @Router /api/v1/musics [get]
func (h *MusicHandler) Search(c *gin.Context) {
	params, err := parseMusicSearchParams(c)
	if err != nil {
		BadRequest(c, err.Error())
		return
	}

	currentUserID := h.getUserID(c)
	if params.LikedOnly && currentUserID == nil {
		Unauthorized(c, "Login is required to filter liked music")
		return
	}
	params.LikedByUserID = currentUserID
	musics, total, err := h.service.Search(c.Request.Context(), params, currentUserID)
	if err != nil {
		InternalServerError(c, "Failed to search music")
		return
	}

	Success(c, gin.H{
		"items": musics,
		"total": total,
		"page":  params.Page,
		"size":  params.PageSize,
	})
}

func (h *MusicHandler) FilterOptions(c *gin.Context) {
	options, err := h.service.ListFilterOptions(c.Request.Context())
	if err != nil {
		InternalServerError(c, "Failed to load music filter options")
		return
	}
	Success(c, options)
}

func (h *MusicHandler) CheckDuplicates(c *gin.Context) {
	var req domain.MusicDuplicateCheckRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, "Invalid duplicate check parameters")
		return
	}

	result, err := h.service.CheckDuplicates(
		c.Request.Context(),
		c.GetUint("userID"),
		c.GetString("role"),
		&req,
	)
	if err != nil {
		InternalServerError(c, "Failed to check duplicate music")
		return
	}
	Success(c, result)
}

// Update godoc
// @Summary 更新音乐
// @Tags musics
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "音乐ID"
// @Param request body domain.UpdateMusicRequest true "更新信息"
// @Success 200 {object} Response "更新成功"
// @Router /api/v1/musics/{id} [put]
func (h *MusicHandler) Update(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		BadRequest(c, "Invalid music ID")
		return
	}

	var req domain.UpdateMusicRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, "Invalid request parameters")
		return
	}

	music, err := h.service.Update(c.Request.Context(), c.GetUint("userID"), c.GetString("role"), uint(id), &req)
	if err != nil {
		if errors.Is(err, service.ErrForbidden) {
			Forbidden(c, "You can only update your own music")
			return
		}
		InternalServerError(c, "Failed to update music")
		return
	}

	Success(c, music)
}

// Delete godoc
// @Summary 删除音乐
// @Tags musics
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "音乐ID"
// @Success 200 {object} Response "删除成功"
// @Router /api/v1/musics/{id} [delete]
func (h *MusicHandler) Delete(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		BadRequest(c, "Invalid music ID")
		return
	}

	if err := h.service.Delete(c.Request.Context(), c.GetUint("userID"), c.GetString("role"), uint(id)); err != nil {
		if errors.Is(err, service.ErrForbidden) {
			Forbidden(c, "You can only delete your own music")
			return
		}
		InternalServerError(c, "Failed to delete music")
		return
	}

	Success(c, gin.H{"message": "Music deleted successfully"})
}

// Like godoc
// @Summary 收藏/喜欢音乐
// @Tags musics
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "音乐ID"
// @Success 200 {object} Response "操作成功"
// @Router /api/v1/musics/{id}/like [post]
func (h *MusicHandler) Like(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		BadRequest(c, "Invalid music ID")
		return
	}

	userID := c.GetUint("userID")
	if err := h.service.Like(c.Request.Context(), userID, uint(id)); err != nil {
		if errors.Is(err, repository.ErrMusicNotFound) {
			NotFound(c, "Music not found")
			return
		}
		InternalServerError(c, "Failed to like music")
		return
	}

	Success(c, gin.H{"message": "Music liked successfully"})
}

// Unlike godoc
// @Summary 取消收藏/喜欢音乐
// @Tags musics
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "音乐ID"
// @Success 200 {object} Response "操作成功"
// @Router /api/v1/musics/{id}/like [delete]
func (h *MusicHandler) Unlike(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		BadRequest(c, "Invalid music ID")
		return
	}

	userID := c.GetUint("userID")
	if err := h.service.Unlike(c.Request.Context(), userID, uint(id)); err != nil {
		InternalServerError(c, "Failed to unlike music")
		return
	}

	Success(c, gin.H{"message": "Music unliked successfully"})
}

// ListUserMusic godoc
// @Summary 获取用户上传的音乐
// @Tags musics
// @Accept json
// @Produce json
// @Param id path int true "用户ID"
// @Param page query int false "页码"
// @Param page_size query int false "每页数量"
// @Success 200 {object} Response "获取成功"
// @Router /api/v1/users/{id}/musics [get]
func (h *MusicHandler) ListUserMusic(c *gin.Context) {
	userID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		BadRequest(c, "Invalid user ID")
		return
	}

	page, pageSize := parsePagination(c, 10)

	currentUserID := h.getUserID(c)
	musics, total, err := h.service.ListByUserID(c.Request.Context(), uint(userID), page, pageSize, currentUserID)
	if err != nil {
		InternalServerError(c, "Failed to list user music")
		return
	}

	Success(c, gin.H{
		"items": musics,
		"total": total,
		"page":  page,
		"size":  pageSize,
	})
}

// ListUserLikedMusic godoc
// @Summary 获取用户喜欢的音乐
// @Tags musics
// @Accept json
// @Produce json
// @Param id path int true "用户ID"
// @Param page query int false "页码"
// @Param page_size query int false "每页数量"
// @Success 200 {object} Response "获取成功"
// @Router /api/v1/users/{id}/likes [get]
func (h *MusicHandler) ListUserLikedMusic(c *gin.Context) {
	userID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		BadRequest(c, "Invalid user ID")
		return
	}

	page, pageSize := parsePagination(c, 10)

	currentUserID := h.getUserID(c)
	musics, total, err := h.service.ListLikedByUserID(c.Request.Context(), uint(userID), page, pageSize, currentUserID)
	if err != nil {
		InternalServerError(c, "Failed to list liked music")
		return
	}

	Success(c, gin.H{
		"items": musics,
		"total": total,
		"page":  page,
		"size":  pageSize,
	})
}

func (h *MusicHandler) getUserID(c *gin.Context) *uint {
	if val, exists := c.Get("userID"); exists {
		if id, ok := val.(uint); ok {
			return &id
		}
	}
	return nil
}

func (h *MusicHandler) Stream(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		BadRequest(c, "Invalid music ID")
		return
	}

	audioPath, err := h.service.GetAudioPath(c.Request.Context(), uint(id))
	if err != nil {
		NotFound(c, "No audio file available")
		return
	}

	h.serveMediaFile(c, audioPath, "Audio file not found on disk")
}

func (h *MusicHandler) Cover(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		BadRequest(c, "Invalid music ID")
		return
	}

	coverPath, err := h.service.GetCoverPath(c.Request.Context(), uint(id))
	if err != nil {
		NotFound(c, "No cover file available")
		return
	}

	h.serveMediaFile(c, coverPath, "Cover file not found on disk")
}

func parseMusicSearchParams(c *gin.Context) (*domain.MusicSearchParams, error) {
	page, pageSize := parsePagination(c, 10)
	params := &domain.MusicSearchParams{
		Query:    c.Query("q"),
		Artist:   c.Query("artist"),
		Page:     page,
		PageSize: pageSize,
	}

	if rawYear := c.Query("year"); rawYear != "" {
		year, err := strconv.Atoi(rawYear)
		if err != nil || year < 1000 || year > 9999 {
			return nil, fmt.Errorf("invalid year filter")
		}
		params.Year = &year
	}

	if rawType := c.Query("type"); rawType != "" {
		musicType := domain.MusicType(rawType)
		if musicType != domain.MusicTypeSingle && musicType != domain.MusicTypeAlbum {
			return nil, fmt.Errorf("invalid music type filter")
		}
		params.Type = &musicType
	}

	if rawLiked := c.Query("liked"); rawLiked != "" {
		liked, err := strconv.ParseBool(rawLiked)
		if err != nil {
			return nil, fmt.Errorf("invalid liked filter")
		}
		params.LikedOnly = liked
	}

	return params, nil
}

func (h *MusicHandler) serveMediaFile(c *gin.Context, mediaPath string, missingMessage string) {
	file, err := os.Open(mediaPath)
	if err != nil {
		NotFound(c, missingMessage)
		return
	}
	defer func() {
		fileErr := file.Close()
		if fileErr != nil {
			pklog.Errorf("Failed to close media file %s: %v", mediaPath, fileErr)
		}
	}()

	stat, err := file.Stat()
	if err != nil {
		InternalServerError(c, "Failed to read file info")
		return
	}

	ext := filepath.Ext(mediaPath)
	contentType := mime.TypeByExtension(ext)
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	c.Header("Content-Type", contentType)
	c.Header("Accept-Ranges", "bytes")

	http.ServeContent(c.Writer, c.Request, stat.Name(), stat.ModTime(), file)
}
