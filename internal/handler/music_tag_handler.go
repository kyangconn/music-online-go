// Package handler music_tag_handler.go - 音乐标签处理器
// 该文件包含音乐标签相关的HTTP处理器，提供RESTful API接口
package handler

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/kyangconn/music-online-go/internal/domain"
	"github.com/kyangconn/music-online-go/internal/repository"
	"github.com/kyangconn/music-online-go/internal/service"
)

// MusicTagHandler 音乐标签处理器
// 负责处理音乐标签相关的HTTP请求，包括创建、查询、更新、删除等操作
type MusicTagHandler struct {
	service service.MusicTagService
}

// MatchTagResponse 标签匹配响应
// 仅 handler 层使用，封装匹配结果
type MatchTagResponse struct {
	IsMatched bool                     `json:"is_matched"`
	Tag       *domain.MusicTagResponse `json:"tag,omitempty"`
}

func NewMusicTagHandler(svc service.MusicTagService) *MusicTagHandler {
	return &MusicTagHandler{service: svc}
}

// CreateMusicTag 创建新的音乐标签（支持无文件反向上传）
// POST /api/v1/music-tags
func (h *MusicTagHandler) CreateMusicTag(c *gin.Context) {
	var req domain.CreateMusicTagRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, err.Error())
		return
	}

	tag, err := h.service.Create(c.Request.Context(), &req)
	if err != nil {
		InternalServerError(c, err.Error())
		return
	}

	Created(c, tag.ToResponse())
}

// GetMusicTag 根据ID获取音乐标签
// GET /api/v1/music-tags/:id
func (h *MusicTagHandler) GetMusicTag(c *gin.Context) {
	id, err := parseUintParam(c, "id")
	if err != nil {
		BadRequest(c, err.Error())
		return
	}

	tag, err := h.service.GetByID(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, repository.ErrMusicTagNotFound) {
			NotFound(c, "Tag not found")
			return
		}
		InternalServerError(c, err.Error())
		return
	}

	Success(c, tag)
}

// UpdateMusicTag 更新现有的音乐标签
// PUT /api/v1/music-tags/:id
func (h *MusicTagHandler) UpdateMusicTag(c *gin.Context) {
	id, err := parseUintParam(c, "id")
	if err != nil {
		BadRequest(c, err.Error())
		return
	}

	var req domain.UpdateMusicTagRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, err.Error())
		return
	}

	tag, err := h.service.Update(c.Request.Context(), id, &req)
	if err != nil {
		InternalServerError(c, err.Error())
		return
	}

	Success(c, tag.ToResponse())
}

// DeleteMusicTag 根据ID删除音乐标签
// DELETE /api/v1/music-tags/:id
func (h *MusicTagHandler) DeleteMusicTag(c *gin.Context) {
	id, err := parseUintParam(c, "id")
	if err != nil {
		BadRequest(c, err.Error())
		return
	}

	if err := h.service.Delete(c.Request.Context(), id); err != nil {
		InternalServerError(c, err.Error())
		return
	}

	Success(c, gin.H{"message": "Tag deleted successfully"})
}

// SearchMusicTags 搜索音乐标签（支持分页）
// POST /api/v1/music-tags/search
func (h *MusicTagHandler) SearchMusicTags(c *gin.Context) {
	var params domain.TagSearchParams
	if err := c.ShouldBindJSON(&params); err != nil {
		BadRequest(c, err.Error())
		return
	}

	tags, total, err := h.service.Search(c.Request.Context(), &params)
	if err != nil {
		InternalServerError(c, err.Error())
		return
	}

	Success(c, gin.H{
		"tags":   tags,
		"total":  total,
		"limit":  params.Limit,
		"offset": params.Offset,
	})
}

// MatchTags 尝试匹配传入的标签数据与现有标签
// POST /api/v1/music-tags/match
func (h *MusicTagHandler) MatchTags(c *gin.Context) {
	var req domain.CreateMusicTagRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, err.Error())
		return
	}

	matchedTag, isMatched, err := h.service.MatchTags(c.Request.Context(), &req)
	if err != nil {
		InternalServerError(c, err.Error())
		return
	}

	response := MatchTagResponse{IsMatched: isMatched}
	if matchedTag != nil {
		response.Tag = matchedTag.ToResponse()
	}

	Success(c, response)
}

// SearchTracks MusicBee兼容的搜索端点
// POST /api/v1/track/search
func (h *MusicTagHandler) SearchTracks(c *gin.Context) {
	var params domain.TagSearchParams
	if err := c.ShouldBindJSON(&params); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	tags, total, err := h.service.Search(c.Request.Context(), &params)
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
func (h *MusicTagHandler) SubmitTrack(c *gin.Context) {
	var req domain.CreateMusicTagRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	tag, err := h.service.Create(c.Request.Context(), &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, tag.ToResponse())
}

// LookupByMBID 根据MusicBrainz ID查找音乐标签
// GET /api/v1/mbid/lookup?musicbrainz_id={id}
func (h *MusicTagHandler) LookupByMBID(c *gin.Context) {
	mbid := c.Query("musicbrainz_id")
	if mbid == "" {
		BadRequest(c, "musicbrainz_id parameter required")
		return
	}

	tag, err := h.service.GetByMusicBrainzID(c.Request.Context(), mbid)
	if err != nil {
		if errors.Is(err, repository.ErrMusicTagNotFound) {
			NotFound(c, "Tag not found")
			return
		}
		InternalServerError(c, err.Error())
		return
	}

	Success(c, tag)
}

// parseUintParam 解析URL路径中的无符号整数参数
func parseUintParam(c *gin.Context, name string) (uint, error) {
	param := c.Param(name)
	id, err := strconv.ParseUint(param, 10, 32)
	if err != nil {
		return 0, err
	}
	return uint(id), nil
}
