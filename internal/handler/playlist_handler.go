package handler

import (
	"errors"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/kyangconn/music-online-go/internal/domain"
	"github.com/kyangconn/music-online-go/internal/repository"
	"github.com/kyangconn/music-online-go/internal/service"
)

type PlaylistHandler struct {
	service service.PlaylistService
}

func NewPlaylistHandler(playlistService service.PlaylistService) *PlaylistHandler {
	return &PlaylistHandler{service: playlistService}
}

func (h *PlaylistHandler) Create(c *gin.Context) {
	var req domain.CreatePlaylistRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, "Invalid playlist parameters")
		return
	}
	playlist, err := h.service.Create(c.Request.Context(), c.GetUint("userID"), &req)
	if err != nil {
		handlePlaylistError(c, err)
		return
	}
	Created(c, playlist)
}

func (h *PlaylistHandler) List(c *gin.Context) {
	page, pageSize := parsePagination(c, 20)
	items, total, err := h.service.List(c.Request.Context(), c.GetUint("userID"), page, pageSize)
	if err != nil {
		InternalServerError(c, "Failed to list playlists")
		return
	}
	Success(c, gin.H{"items": items, "total": total, "page": page, "size": pageSize})
}

func (h *PlaylistHandler) Get(c *gin.Context) {
	id, ok := playlistPathID(c, "id")
	if !ok {
		return
	}
	playlist, err := h.service.Get(c.Request.Context(), c.GetUint("userID"), id)
	if err != nil {
		handlePlaylistError(c, err)
		return
	}
	Success(c, playlist)
}

func (h *PlaylistHandler) Update(c *gin.Context) {
	id, ok := playlistPathID(c, "id")
	if !ok {
		return
	}
	var req domain.UpdatePlaylistRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, "Invalid playlist parameters")
		return
	}
	playlist, err := h.service.Update(c.Request.Context(), c.GetUint("userID"), id, &req)
	if err != nil {
		handlePlaylistError(c, err)
		return
	}
	Success(c, playlist)
}

func (h *PlaylistHandler) Delete(c *gin.Context) {
	id, ok := playlistPathID(c, "id")
	if !ok {
		return
	}
	if err := h.service.Delete(c.Request.Context(), c.GetUint("userID"), id); err != nil {
		handlePlaylistError(c, err)
		return
	}
	Success(c, gin.H{"message": "Playlist deleted successfully"})
}

func (h *PlaylistHandler) AddItem(c *gin.Context) {
	id, ok := playlistPathID(c, "id")
	if !ok {
		return
	}
	var req domain.AddPlaylistItemRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, "Invalid playlist item parameters")
		return
	}
	playlist, err := h.service.AddItem(c.Request.Context(), c.GetUint("userID"), id, req.MusicID)
	if err != nil {
		handlePlaylistError(c, err)
		return
	}
	Success(c, playlist)
}

func (h *PlaylistHandler) RemoveItem(c *gin.Context) {
	id, ok := playlistPathID(c, "id")
	if !ok {
		return
	}
	musicID, ok := playlistPathID(c, "musicID")
	if !ok {
		return
	}
	playlist, err := h.service.RemoveItem(c.Request.Context(), c.GetUint("userID"), id, musicID)
	if err != nil {
		handlePlaylistError(c, err)
		return
	}
	Success(c, playlist)
}

func (h *PlaylistHandler) ReorderItems(c *gin.Context) {
	id, ok := playlistPathID(c, "id")
	if !ok {
		return
	}
	var req domain.ReorderPlaylistItemsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, "Invalid playlist order")
		return
	}
	playlist, err := h.service.ReorderItems(c.Request.Context(), c.GetUint("userID"), id, req.MusicIDs)
	if err != nil {
		handlePlaylistError(c, err)
		return
	}
	Success(c, playlist)
}

func playlistPathID(c *gin.Context, name string) (uint, bool) {
	id, err := strconv.ParseUint(c.Param(name), 10, 32)
	if err != nil || id == 0 {
		BadRequest(c, "Invalid playlist identifier")
		return 0, false
	}
	return uint(id), true
}

func handlePlaylistError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrInvalidPlaylist),
		errors.Is(err, repository.ErrPlaylistItemsMismatch):
		BadRequest(c, err.Error())
	case errors.Is(err, repository.ErrPlaylistNotFound):
		NotFound(c, "Playlist not found")
	case errors.Is(err, repository.ErrPlaylistItemNotFound):
		NotFound(c, "Playlist item not found")
	case errors.Is(err, repository.ErrMusicNotFound):
		NotFound(c, "Music not found")
	case errors.Is(err, repository.ErrPlaylistFull):
		Error(c, 422, "Playlist item limit reached")
	default:
		InternalServerError(c, "Playlist operation failed")
	}
}
