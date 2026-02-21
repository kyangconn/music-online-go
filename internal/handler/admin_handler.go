package handler

import (
	"net/http"
	"os"
	"runtime"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/kyangconn/music-online-web/internal/config"
	"github.com/kyangconn/music-online-web/internal/pkg/database"
	"github.com/kyangconn/music-online-web/internal/service"
)

type AdminHandler struct {
	userService  service.UserService
	musicService service.MusicService
}

func NewAdminHandler(userService service.UserService, musicService service.MusicService) *AdminHandler {
	return &AdminHandler{
		userService:  userService,
		musicService: musicService,
	}
}

type SystemInfoResponse struct {
	Host            string `json:"host"`
	ServerMode      string `json:"server_mode"`
	ServerPort      string `json:"server_port"`
	AppTime         string `json:"app_time"`
	GoVersion       string `json:"go_version"`
	MemoryAlloc     uint64 `json:"memory_alloc"`
	MemorySys       uint64 `json:"memory_sys"`
	Goroutines      int    `json:"goroutines"`
	DBMaxOpenConns  int    `json:"db_max_open_conns"`
	DBOpenConns     int    `json:"db_open_conns"`
	DBInUse         int    `json:"db_in_use"`
	DBIdle          int    `json:"db_idle"`
	TotalMusicCount int64  `json:"total_music_count"`
}

func (h *AdminHandler) SystemInfo(c *gin.Context) {
	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)

	sqlDB, err := database.DB.DB()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "database not initialized"})
		return
	}
	dbStats := sqlDB.Stats()

	host := c.Request.Host
	if host == "" {
		host = os.Getenv("HOSTNAME")
	}

	_, total, err := h.musicService.Search("", 1, 1, nil)
	if err != nil {
		InternalServerError(c, "Failed to count musics")
		return
	}

	info := SystemInfoResponse{
		Host:            host,
		ServerMode:      config.AppConfig.Server.Mode,
		ServerPort:      config.AppConfig.Server.Port,
		AppTime:         time.Now().Format(time.RFC3339),
		GoVersion:       runtime.Version(),
		MemoryAlloc:     mem.Alloc,
		MemorySys:       mem.Sys,
		Goroutines:      runtime.NumGoroutine(),
		DBMaxOpenConns:  dbStats.MaxOpenConnections,
		DBOpenConns:     dbStats.OpenConnections,
		DBInUse:         dbStats.InUse,
		DBIdle:          dbStats.Idle,
		TotalMusicCount: total,
	}

	Success(c, info)
}

// ListUsers GoDoc
// @Summary 管理员获取用户列表
// @Description 支持搜索和分页
// @Tags admin
// @Accept json
// @Produce json
// @Param q query string false "搜索关键词"
// @Param page query int false "页码"
// @Param page_size query int false "每页数量"
// @Security BearerAuth
// @Success 200 {object} Response
// @Router /api/v1/admin/users [get]
func (h *AdminHandler) ListUsers(c *gin.Context) {
	query := c.Query("q")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "10"))

	users, total, err := h.userService.SearchUsers(query, page, pageSize)
	if err != nil {
		InternalServerError(c, "Failed to fetch users")
		return
	}

	Success(c, gin.H{
		"items": users,
		"total": total,
		"page":  page,
		"size":  pageSize,
	})
}

type UpdateStatusRequest struct {
	IsActive bool `json:"is_active"`
}

// UpdateUserStatus GoDoc
// @Summary 封禁/解封用户
// @Tags admin
// @Accept JSON
// @Produce JSON
// @Param id path int true "用户ID"
// @Param request body UpdateStatusRequest true "状态信息"
// @Security BearerAuth
// @Success 200 {object} Response
// @Router /api/v1/admin/users/{id}/status [put]
func (h *AdminHandler) UpdateUserStatus(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		BadRequest(c, "Invalid user ID")
		return
	}

	var req UpdateStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, "Invalid request body")
		return
	}

	if err := h.userService.UpdateUserStatus(uint(id), req.IsActive); err != nil {
		InternalServerError(c, "Failed to update user status")
		return
	}

	Success(c, nil)
}

type UpdateRoleRequest struct {
	Role string `json:"role" binding:"required,oneof=user admin"`
}

// UpdateUserRole GoDoc
// @Summary 修改用户角色
// @Tags admin
// @Accept JSON
// @Produce JSON
// @Param id path int true "用户ID"
// @Param request body UpdateRoleRequest true "角色信息"
// @Security BearerAuth
// @Success 200 {object} Response
// @Router /api/v1/admin/users/{id}/role [put]
func (h *AdminHandler) UpdateUserRole(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		BadRequest(c, "Invalid user ID")
		return
	}

	var req UpdateRoleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, "Invalid request body")
		return
	}

	if err := h.userService.UpdateUserRole(uint(id), req.Role); err != nil {
		InternalServerError(c, "Failed to update user role")
		return
	}

	Success(c, nil)
}

// DeleteMusic GoDoc
// @Summary 管理员删除音乐
// @Tags admin
// @Accept JSON
// @Produce JSON
// @Param id path int true "音乐ID"
// @Security BearerAuth
// @Success 200 {object} Response
// @Router /api/v1/admin/musics/{id} [delete]
func (h *AdminHandler) DeleteMusic(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		BadRequest(c, "Invalid music ID")
		return
	}

	if err := h.musicService.AdminDelete(uint(id)); err != nil {
		if err.Error() == "music not found" {
			NotFound(c, "Music not found")
		} else {
			InternalServerError(c, "Failed to delete music")
		}
		return
	}

	Success(c, nil)
}
