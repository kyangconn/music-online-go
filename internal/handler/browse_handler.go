package handler

import (
	"errors"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/kyangconn/music-online-go/internal/domain"
	"github.com/kyangconn/music-online-go/internal/repository"
	"github.com/kyangconn/music-online-go/internal/service"
)

type BrowseHandler struct {
	service service.BrowseService
}

func NewBrowseHandler(browseService service.BrowseService) *BrowseHandler {
	return &BrowseHandler{service: browseService}
}

func (h *BrowseHandler) ListArtists(c *gin.Context) {
	page, pageSize := parsePagination(c, 24)
	items, total, err := h.service.ListArtists(c.Request.Context(), domain.BrowseArtistParams{
		Query: c.Query("q"), Page: page, PageSize: pageSize,
	})
	if err != nil {
		InternalServerError(c, "Failed to list artists")
		return
	}
	Success(c, gin.H{"items": items, "total": total, "page": page, "size": pageSize})
}

func (h *BrowseHandler) GetArtist(c *gin.Context) {
	artist, err := h.service.GetArtist(c.Request.Context(), c.Param("key"))
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidBrowseKey):
			BadRequest(c, "Invalid artist key")
		case errors.Is(err, repository.ErrArtistNotFound):
			NotFound(c, "Artist not found")
		default:
			InternalServerError(c, "Failed to load artist")
		}
		return
	}
	Success(c, artist)
}

func (h *BrowseHandler) ListAlbums(c *gin.Context) {
	page, pageSize := parsePagination(c, 24)
	params := domain.BrowseAlbumParams{
		Query: c.Query("q"), ArtistKey: c.Query("artist_key"), AlbumArtist: c.Query("album_artist"),
		Genre: c.Query("genre"), Page: page, PageSize: pageSize,
	}
	if rawYear := c.Query("year"); rawYear != "" {
		year, err := strconv.Atoi(rawYear)
		if err != nil || year < 1000 || year > 9999 {
			BadRequest(c, "Invalid year filter")
			return
		}
		params.Year = &year
	}
	items, total, err := h.service.ListAlbums(c.Request.Context(), params)
	if err != nil {
		if errors.Is(err, service.ErrInvalidBrowseKey) {
			BadRequest(c, "Invalid artist key")
			return
		}
		InternalServerError(c, "Failed to list albums")
		return
	}
	Success(c, gin.H{"items": items, "total": total, "page": page, "size": pageSize})
}

func (h *BrowseHandler) GetAlbum(c *gin.Context) {
	album, err := h.service.GetAlbum(c.Request.Context(), c.Param("key"))
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidBrowseKey):
			BadRequest(c, "Invalid album key")
		case errors.Is(err, repository.ErrAlbumNotFound):
			NotFound(c, "Album not found")
		default:
			InternalServerError(c, "Failed to load album")
		}
		return
	}
	Success(c, album)
}
