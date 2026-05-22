// Package handler admin_handler.go - 管理后台处理器
// 提供系统信息、用户管理、音乐管理等管理员接口
package handler

import (
	"fmt"
	"net/http"
	"os"
	"runtime"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/kyangconn/music-online-go/internal/config"
	"github.com/kyangconn/music-online-go/internal/pkg/database"
	"github.com/kyangconn/music-online-go/internal/service"
	"github.com/kyangconn/music-online-go/internal/version"
)

// startTime 记录进程启动时间
var startTime = time.Now()

// AdminHandler handles admin-related HTTP requests.
type AdminHandler struct {
	userService     service.UserService
	musicService    service.MusicService
	musicTagService *service.MusicTagService
}

func NewAdminHandler(userService service.UserService, musicService service.MusicService, musicTagService *service.MusicTagService) *AdminHandler {
	return &AdminHandler{
		userService:     userService,
		musicService:    musicService,
		musicTagService: musicTagService,
	}
}

type SystemInfoResponse struct {
	Host       string `json:"host"`
	ServerMode string `json:"server_mode"`
	ServerPort string `json:"server_port"`
	AppVersion string `json:"app_version"`
	AppCommit  string `json:"app_commit"`
	AppBuilt   string `json:"app_built"`
	AppTime    string `json:"app_time"`
	Uptime     string `json:"uptime"`

	GoVersion  string `json:"go_version"`
	NumCPU     int    `json:"num_cpu"`
	Goroutines int    `json:"goroutines"`

	MemoryAlloc      string `json:"memory_alloc"`
	MemoryTotalAlloc string `json:"memory_total_alloc"`
	MemorySys        string `json:"memory_sys"`
	HeapAlloc        string `json:"heap_alloc"`
	HeapSys          string `json:"heap_sys"`
	HeapIdle         string `json:"heap_idle"`
	HeapInuse        string `json:"heap_inuse"`
	HeapReleased     string `json:"heap_released"`
	HeapObjects      uint64 `json:"heap_objects"`
	StackInuse       string `json:"stack_inuse"`
	StackSys         string `json:"stack_sys"`

	NumGC      uint32 `json:"num_gc"`
	PauseTotal string `json:"pause_total"`
	LastGCTime string `json:"last_gc_time"`
	GCCPUFrac  string `json:"gc_cpu_fraction"`

	DBMaxOpenConns int    `json:"db_max_open_conns"`
	DBOpenConns    int    `json:"db_open_conns"`
	DBInUse        int    `json:"db_in_use"`
	DBIdle         int    `json:"db_idle"`
	DBWaitCount    int64  `json:"db_wait_count"`
	DBWaitDuration string `json:"db_wait_duration"`
	DBType         string `json:"db_type"`
	DBName         string `json:"db_name"`

	TotalUsers     int64 `json:"total_users"`
	TotalMusic     int64 `json:"total_music"`
	TotalMusicTags int64 `json:"total_music_tags"`
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

	uptime := time.Since(startTime).Round(time.Second).String()

	gcCPUFrac := 0.0
	if mem.PauseTotalNs > 0 {
		gcCPUFrac = float64(mem.PauseTotalNs) / float64(time.Since(startTime).Nanoseconds()) * 100
	}

	info := SystemInfoResponse{
		Host:       host,
		ServerMode: config.AppConfig.Server.Mode,
		ServerPort: config.AppConfig.Server.Port,
		AppVersion: version.Version,
		AppCommit:  version.Commit,
		AppBuilt:   version.BuildTime,
		AppTime:    time.Now().Format(time.RFC3339),
		Uptime:     uptime,

		GoVersion:  runtime.Version(),
		NumCPU:     runtime.NumCPU(),
		Goroutines: runtime.NumGoroutine(),

		MemoryAlloc:      formatBytes(mem.Alloc),
		MemoryTotalAlloc: formatBytes(mem.TotalAlloc),
		MemorySys:        formatBytes(mem.Sys),
		HeapAlloc:        formatBytes(mem.HeapAlloc),
		HeapSys:          formatBytes(mem.HeapSys),
		HeapIdle:         formatBytes(mem.HeapIdle),
		HeapInuse:        formatBytes(mem.HeapInuse),
		HeapReleased:     formatBytes(mem.HeapReleased),
		HeapObjects:      mem.HeapObjects,
		StackInuse:       formatBytes(mem.StackInuse),
		StackSys:         formatBytes(mem.StackSys),

		NumGC:      mem.NumGC,
		PauseTotal: fmtDuration(time.Duration(mem.PauseTotalNs)),
		LastGCTime: time.Unix(0, int64(mem.LastGC)).Format(time.RFC3339),
		GCCPUFrac:  fmt.Sprintf("%.4f%%", gcCPUFrac),

		DBMaxOpenConns: dbStats.MaxOpenConnections,
		DBOpenConns:    dbStats.OpenConnections,
		DBInUse:        dbStats.InUse,
		DBIdle:         dbStats.Idle,
		DBWaitCount:    dbStats.WaitCount,
		DBWaitDuration: fmtDuration(dbStats.WaitDuration),
		DBType:         config.AppConfig.Database.Type,
		DBName:         config.AppConfig.Database.Name,
	}

	if total, err := h.userService.CountAll(); err == nil {
		info.TotalUsers = total
	}

	if _, total, err := h.musicService.Search("", 1, 1, nil); err == nil {
		info.TotalMusic = total
	}

	if total, err := h.musicTagService.CountAll(); err == nil {
		info.TotalMusicTags = total
	}

	Success(c, info)
}

func formatBytes(bytes uint64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := uint64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

func fmtDuration(d time.Duration) string {
	if d < time.Microsecond {
		return fmt.Sprintf("%d ns", d.Nanoseconds())
	}
	return d.Round(time.Microsecond).String()
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
