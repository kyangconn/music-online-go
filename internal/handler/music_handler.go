package handler

import (
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/kyangconn/music-online-go/internal/domain"
	"github.com/kyangconn/music-online-go/internal/service"
)

type MusicHandler struct {
	musicService service.MusicService
}

func NewMusicHandler(musicService service.MusicService) *MusicHandler {
	return &MusicHandler{musicService: musicService}
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
	music, err := h.musicService.Create(userID, &req)
	if err != nil {
		InternalServerError(c, "Failed to create music")
		return
	}

	Created(c, music)
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
	music, err := h.musicService.GetByID(uint(id), currentUserID)
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
	query := c.Query("q")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "10"))

	currentUserID := h.getUserID(c)
	musics, total, err := h.musicService.Search(query, page, pageSize, currentUserID)
	if err != nil {
		InternalServerError(c, "Failed to search music")
		return
	}

	Success(c, gin.H{
		"items": musics,
		"total": total,
		"page":  page,
		"size":  pageSize,
	})
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

	// TODO: Add ownership check here or in service

	music, err := h.musicService.Update(uint(id), &req)
	if err != nil {
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

	if err := h.musicService.Delete(uint(id)); err != nil {
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
	if err := h.musicService.Like(userID, uint(id)); err != nil {
		if err.Error() == "music not found" {
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
	if err := h.musicService.Unlike(userID, uint(id)); err != nil {
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

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "10"))

	currentUserID := h.getUserID(c)
	musics, total, err := h.musicService.ListByUserID(uint(userID), page, pageSize, currentUserID)
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

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "10"))

	currentUserID := h.getUserID(c)
	musics, total, err := h.musicService.ListLikedByUserID(uint(userID), page, pageSize, currentUserID)
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
