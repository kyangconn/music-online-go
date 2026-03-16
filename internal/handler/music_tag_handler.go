// music_tag_handler.go - 音乐标签处理器
// 该文件包含音乐标签相关的HTTP处理器，提供RESTful API接口
package handler

import (
	"net/http"
	"strconv"

	"github.com/kyangconn/music-online-go/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/kyangconn/music-online-go/internal/domain"
)

// MusicTagHandler 音乐标签处理器
// 负责处理音乐标签相关的HTTP请求，包括创建、查询、更新、删除等操作
type MusicTagHandler struct {
	service *service.MusicTagService
}

// DTOs for API requests/responses

// MusicTagResponse 音乐标签响应DTO
// 用于API响应的音乐标签数据结构
type MusicTagResponse struct {
	ID                  uint   `json:"id"`
	Artist              string `json:"artist"`
	Title               string `json:"title"`
	Album               string `json:"album"`
	AlbumArtist         string `json:"album_artist"`
	TrackNumber         *int   `json:"track_number"`
	DiscNumber          *int   `json:"disc_number"`
	Genre               string `json:"genre"`
	Year                *int   `json:"year"`
	Duration            *int   `json:"duration"`
	Comment             string `json:"comment"`
	MusicBrainzID       string `json:"musicbrainz_id"`
	MusicBrainzArtistID string `json:"musicbrainz_artist_id"`
}

type CreateMusicTagRequest struct {
	Artist              string `json:"artist" binding:"required"`
	Title               string `json:"title" binding:"required"`
	Album               string `json:"album"`
	AlbumArtist         string `json:"album_artist"`
	TrackNumber         *int   `json:"track_number"`
	DiscNumber          *int   `json:"disc_number"`
	Genre               string `json:"genre"`
	Year                *int   `json:"year"`
	Duration            *int   `json:"duration"`
	Comment             string `json:"comment"`
	MusicBrainzID       string `json:"musicbrainz_id"`
	MusicBrainzArtistID string `json:"musicbrainz_artist_id"`
}

type UpdateMusicTagRequest struct {
	Artist              *string `json:"artist"`
	Title               *string `json:"title"`
	Album               *string `json:"album"`
	AlbumArtist         *string `json:"album_artist"`
	TrackNumber         *int    `json:"track_number"`
	DiscNumber          *int    `json:"disc_number"`
	Genre               *string `json:"genre"`
	Year                *int    `json:"year"`
	Duration            *int    `json:"duration"`
	Comment             *string `json:"comment"`
	MusicBrainzID       *string `json:"musicbrainz_id"`
	MusicBrainzArtistID *string `json:"musicbrainz_artist_id"`
}

type TagSearchParams struct {
	Artist string `json:"artist"`
	Title  string `json:"title"`
	Album  string `json:"album"`
	Genre  string `json:"genre"`
	Year   *int   `json:"year"`
	Limit  int    `json:"limit"`
	Offset int    `json:"offset"`
}

type TrackSearchParams struct {
	Artist string `json:"artist"`
	Title  string `json:"title"`
	Album  string `json:"album"`
	Genre  string `json:"genre"`
	Year   *int   `json:"year"`
	Limit  int    `json:"limit"`
	Offset int    `json:"offset"`
}

type TrackSubmitRequest struct {
	Artist              string `json:"artist" binding:"required"`
	Title               string `json:"title" binding:"required"`
	Album               string `json:"album"`
	AlbumArtist         string `json:"album_artist"`
	TrackNumber         *int   `json:"track_number"`
	DiscNumber          *int   `json:"disc_number"`
	Genre               string `json:"genre"`
	Year                *int   `json:"year"`
	Duration            *int   `json:"duration"`
	Comment             string `json:"comment"`
	MusicBrainzID       string `json:"musicbrainz_id"`
	MusicBrainzArtistID string `json:"musicbrainz_artist_id"`
}

type MatchTagResponse struct {
	IsMatched bool              `json:"is_matched"`
	Tag       *MusicTagResponse `json:"tag,omitempty"`
}

func NewMusicTagHandler(service *service.MusicTagService) *MusicTagHandler {
	return &MusicTagHandler{service: service}
}

// convertDomainTagToResponse 将领域层的标签响应转换为handler层的标签响应
func convertDomainTagToResponse(domainTag *domain.MusicTagResponse) *MusicTagResponse {
	if domainTag == nil {
		return nil
	}

	response := &MusicTagResponse{
		ID:                  domainTag.ID,
		Artist:              domainTag.Artist,
		Title:               domainTag.Title,
		Album:               domainTag.Album,
		AlbumArtist:         domainTag.AlbumArtist,
		Genre:               domainTag.Genre,
		Comment:             domainTag.Comment,
		MusicBrainzID:       domainTag.MusicBrainzID,
		MusicBrainzArtistID: domainTag.MusicBrainzArtistID,
	}

	// 处理指针类型字段
	if domainTag.TrackNumber != 0 {
		trackNum := domainTag.TrackNumber
		response.TrackNumber = &trackNum
	}

	if domainTag.DiscNumber != 0 {
		discNum := domainTag.DiscNumber
		response.DiscNumber = &discNum
	}

	if domainTag.Year != 0 {
		year := domainTag.Year
		response.Year = &year
	}

	if domainTag.Duration != 0 {
		duration := domainTag.Duration
		response.Duration = &duration
	}

	return response
}

// CreateMusicTag 创建新的音乐标签（支持无文件反向上传）
// POST /api/v1/music-tags
// @Summary Create music tag
// @Description Creates a new music tag. Can be used for reverse upload without audio file.
// @Tags music tags
// @Accept json
// @Produce json
// @Param request body CreateMusicTagRequest true "Create music tag request"
// @Success 201 {object} domain.MusicTagResponse
// @Router /api/v1/music-tags [post]
func (h *MusicTagHandler) CreateMusicTag(c *gin.Context) {
	var req CreateMusicTagRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 将API请求转换为领域模型请求
	tagReq := convertCreateRequestToDomain(&req)

	tag, err := h.service.Create(tagReq)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, tag.ToResponse())
}

// convertCreateRequestToDomain 将创建请求转换为领域模型请求
func convertCreateRequestToDomain(req *CreateMusicTagRequest) *domain.CreateMusicTagRequest {
	tagReq := &domain.CreateMusicTagRequest{
		Artist:              req.Artist,
		Title:               req.Title,
		Album:               req.Album,
		AlbumArtist:         req.AlbumArtist,
		Genre:               req.Genre,
		Comment:             req.Comment,
		MusicBrainzID:       req.MusicBrainzID,
		MusicBrainzArtistID: req.MusicBrainzArtistID,
	}

	// 处理可选字段
	if req.TrackNumber != nil {
		tagReq.TrackNumber = req.TrackNumber
	}
	if req.DiscNumber != nil {
		tagReq.DiscNumber = req.DiscNumber
	}
	if req.Year != nil {
		tagReq.Year = req.Year
	}
	if req.Duration != nil {
		tagReq.Duration = req.Duration
	}

	return tagReq
}

// GetMusicTag 根据ID获取音乐标签
// GET /api/v1/music-tags/:id
// @Summary Get music tag
// @Description Retrieves a music tag by its ID.
// @Tags music tags
// @Produce json
// @Param id path uint true "Tag ID"
// @Success 200 {object} domain.MusicTagResponse
// @Router /api/v1/music-tags/{id} [get]
func (h *MusicTagHandler) GetMusicTag(c *gin.Context) {
	id, err := parseUintParam(c, "id")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	tag, err := h.service.GetByID(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Tag not found"})
		return
	}

	c.JSON(http.StatusOK, tag)
}

// UpdateMusicTag 更新现有的音乐标签
// PUT /api/v1/music-tags/:id
// @Summary Update music tag
// @Description Updates an existing music tag.
// @Tags music tags
// @Accept json
// @Produce json
// @Param id path uint true "Tag ID"
// @Param request body UpdateMusicTagRequest true "Update music tag request"
// @Success 200 {object} domain.MusicTagResponse
// @Router /api/v1/music-tags/{id} [put]
func (h *MusicTagHandler) UpdateMusicTag(c *gin.Context) {
	id, err := parseUintParam(c, "id")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var req UpdateMusicTagRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 将API请求转换为领域模型请求
	tagReq := convertUpdateRequestToDomain(&req)

	tag, err := h.service.Update(id, tagReq)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, tag.ToResponse())
}

// convertUpdateRequestToDomain 将更新请求转换为领域模型请求
func convertUpdateRequestToDomain(req *UpdateMusicTagRequest) *domain.UpdateMusicTagRequest {
	tagReq := &domain.UpdateMusicTagRequest{}

	// 处理所有可选字段
	if req.Artist != nil {
		tagReq.Artist = req.Artist
	}
	if req.Title != nil {
		tagReq.Title = req.Title
	}
	if req.Album != nil {
		tagReq.Album = req.Album
	}
	if req.AlbumArtist != nil {
		tagReq.AlbumArtist = req.AlbumArtist
	}
	if req.Genre != nil {
		tagReq.Genre = req.Genre
	}
	if req.Comment != nil {
		tagReq.Comment = req.Comment
	}
	if req.MusicBrainzID != nil {
		tagReq.MusicBrainzID = req.MusicBrainzID
	}
	if req.MusicBrainzArtistID != nil {
		tagReq.MusicBrainzArtistID = req.MusicBrainzArtistID
	}
	if req.TrackNumber != nil {
		tagReq.TrackNumber = req.TrackNumber
	}
	if req.DiscNumber != nil {
		tagReq.DiscNumber = req.DiscNumber
	}
	if req.Year != nil {
		tagReq.Year = req.Year
	}
	if req.Duration != nil {
		tagReq.Duration = req.Duration
	}

	return tagReq
}

// DeleteMusicTag 根据ID删除音乐标签
// DELETE /api/v1/music-tags/:id
// @Summary Delete music tag
// @Description Deletes a music tag by its ID.
// @Tags music tags
// @Produce json
// @Param id path uint true "Tag ID"
// @Success 204 "No Content"
// @Router /api/v1/music-tags/{id} [delete]
func (h *MusicTagHandler) DeleteMusicTag(c *gin.Context) {
	id, err := parseUintParam(c, "id")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	err = h.service.Delete(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.Status(http.StatusNoContent)
}

// SearchMusicTags 搜索音乐标签（支持分页）
// GET /api/v1/music-tags
// @Summary Search music tags
// @Description Searches for music tags with pagination support.
// @Tags music tags
// @Produce json
// @Param artist query string false "Artist name"
// @Param title query string false "Title"
// @Param album query string false "Album name"
// @Param genre query string false "Genre"
// @Param year query int false "Year"
// @Param limit query int false "Limit (default: 20)"
// @Param offset query int false "Offset (default: 0)"
// @Success 200 {object} SearchResponse
// @Router /api/v1/music-tags [get]
func (h *MusicTagHandler) SearchMusicTags(c *gin.Context) {
	var params domain.TagSearchParams
	if err := c.ShouldBindQuery(&params); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	tags, total, err := h.service.Search(&params)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"tags":   tags,
		"total":  total,
		"limit":  params.Limit,
		"offset": params.Offset,
	})
}

// MatchTags 尝试匹配传入的标签数据与现有标签
// POST /api/v1/music-tags/match
// @Summary Match music tags
// @Description Attempts to match incoming tag data with existing tags in the database. Returns matched tag if found.
// @Tags music tags
// @Accept json
// @Produce json
// @Param request body CreateMusicTagRequest true "Tag data to match"
// @Success 200 {object} MatchTagResponse
// @Router /api/v1/music-tags/match [post]
func (h *MusicTagHandler) MatchTags(c *gin.Context) {
	var req CreateMusicTagRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 使用相同的转换函数
	tagReq := convertCreateRequestToDomain(&req)

	matchedTag, isMatched, err := h.service.MatchTags(tagReq)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	response := MatchTagResponse{
		IsMatched: isMatched,
	}

	if matchedTag != nil {
		response.Tag = convertDomainTagToResponse(matchedTag.ToResponse())
	}

	c.JSON(http.StatusOK, response)
}

// SearchTracks MusicBee兼容的搜索端点
// POST /api/v1/track/search
// @Summary Search tracks (MusicBee compatible)
// @Description MusicBee-compatible track search endpoint.
// @Tags music tags
// @Produce json
// @Param request body TrackSearchParams true "Search parameters"
// @Success 200 {array} domain.MusicTagResponse
// @Router /api/v1/track/search [post]
func (h *MusicTagHandler) SearchTracks(c *gin.Context) {
	var params TrackSearchParams
	if err := c.ShouldBindJSON(&params); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	tagParams := &domain.TagSearchParams{
		Artist: params.Artist,
		Title:  params.Title,
		Album:  params.Album,
		Genre:  params.Genre,
		Year:   params.Year,
		Limit:  params.Limit,
		Offset: params.Offset,
	}

	tags, total, err := h.service.Search(tagParams)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"tracks": tags,
		"total":  total,
		"count":  len(tags),
	})
}

// SubmitTrack MusicBee兼容的提交端点
// POST /api/v1/track/submit
// @Summary Submit track metadata (MusicBee compatible)
// @Description MusicBee-compatible track metadata submission. Allows uploading tags without audio file.
// @Tags music tags
// @Accept json
// @Produce json
// @Param request body TrackSubmitRequest true "Track metadata to submit"
// @Success 201 {object} domain.MusicTagResponse
// @Router /api/v1/track/submit [post]
func (h *MusicTagHandler) SubmitTrack(c *gin.Context) {
	var req TrackSubmitRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 将TrackSubmitRequest转换为CreateMusicTagRequest，然后使用相同的转换函数
	createReq := &CreateMusicTagRequest{
		Artist:              req.Artist,
		Title:               req.Title,
		Album:               req.Album,
		AlbumArtist:         req.AlbumArtist,
		Genre:               req.Genre,
		TrackNumber:         req.TrackNumber,
		DiscNumber:          req.DiscNumber,
		Year:                req.Year,
		Duration:            req.Duration,
		Comment:             req.Comment,
		MusicBrainzID:       req.MusicBrainzID,
		MusicBrainzArtistID: req.MusicBrainzArtistID,
	}

	tagReq := convertCreateRequestToDomain(createReq)

	tag, err := h.service.Create(tagReq)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, tag.ToResponse())
}

// LookupByMBID 根据MusicBrainz ID查找音乐标签
// GET /api/v1/mbid/lookup?musicbrainz_id={id}
// @Summary Lookup by MusicBrainz ID
// @Description Look up a music tag by its MusicBrainz ID.
// @Tags music tags
// @Produce json
// @Param musicbrainz_id query string true "MusicBrainz ID"
// @Success 200 {object} domain.MusicTagResponse
// @Router /api/v1/mbid/lookup [get]
func (h *MusicTagHandler) LookupByMBID(c *gin.Context) {
	mbid := c.Query("musicbrainz_id")
	if mbid == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "musicbrainz_id parameter required"})
		return
	}

	tag, err := h.service.Repo.GetByMusicBrainzID(mbid)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Tag not found"})
		return
	}

	response := tag.ToResponse()
	c.JSON(http.StatusOK, response)
}

// Helper functions

// parseUintParam 解析URL路径中的无符号整数参数
// 用于从URL路径中提取ID等参数并转换为uint类型
func parseUintParam(c *gin.Context, name string) (uint, error) {
	param := c.Param(name)
	id, err := strconv.ParseUint(param, 10, 32)
	if err != nil {
		return 0, err
	}
	return uint(id), nil
}
