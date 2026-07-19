// Package handler admin_handler.go - 管理后台处理器
// 提供系统信息、用户管理、音乐管理等管理员接口
package handler

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"runtime"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/kyangconn/music-online-go/internal/config"
	"github.com/kyangconn/music-online-go/internal/domain"
	"github.com/kyangconn/music-online-go/internal/pkg/database"
	"github.com/kyangconn/music-online-go/internal/pkg/password"
	"github.com/kyangconn/music-online-go/internal/repository"
	"github.com/kyangconn/music-online-go/internal/service"
	"github.com/kyangconn/music-online-go/internal/version"
)

// startTime 记录进程启动时间
var startTime = time.Now()

// AdminHandler handles admin-related HTTP requests.
type AdminHandler struct {
	userService         service.UserService
	musicService        service.MusicService
	mediaLibraryService service.MediaLibraryService
	serverConfig        config.ServerConfig
	databaseConfig      config.DatabaseConfig
}

func NewAdminHandler(userService service.UserService, musicService service.MusicService, mediaLibraryServices ...service.MediaLibraryService) *AdminHandler {
	var mediaLibraryService service.MediaLibraryService
	if len(mediaLibraryServices) > 0 {
		mediaLibraryService = mediaLibraryServices[0]
	}
	return NewAdminHandlerWithConfig(userService, musicService, mediaLibraryService, config.AppConfig)
}

// NewAdminHandlerWithConfig keeps system-info output tied to the same
// validated startup snapshot used to construct the rest of the application.
func NewAdminHandlerWithConfig(userService service.UserService, musicService service.MusicService, mediaLibraryService service.MediaLibraryService, cfg *config.Config) *AdminHandler {
	handler := &AdminHandler{
		userService:         userService,
		musicService:        musicService,
		mediaLibraryService: mediaLibraryService,
	}
	if cfg != nil {
		handler.serverConfig = cfg.Server
		handler.databaseConfig = cfg.Database
	}
	return handler
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

	ctx := c.Request.Context()

	info := SystemInfoResponse{
		Host:       host,
		ServerMode: h.serverConfig.Mode,
		ServerPort: h.serverConfig.Port,
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
		PauseTotal: fmtDuration(time.Duration(mem.PauseTotalNs)),         //nolint:gosec // PauseTotalNs < int64 max for realistic uptimes
		LastGCTime: time.Unix(0, int64(mem.LastGC)).Format(time.RFC3339), //nolint:gosec // LastGC timestamp within int64 range until year 2262
		GCCPUFrac:  fmt.Sprintf("%.4f%%", gcCPUFrac),

		DBMaxOpenConns: dbStats.MaxOpenConnections,
		DBOpenConns:    dbStats.OpenConnections,
		DBInUse:        dbStats.InUse,
		DBIdle:         dbStats.Idle,
		DBWaitCount:    dbStats.WaitCount,
		DBWaitDuration: fmtDuration(dbStats.WaitDuration),
		DBType:         h.databaseConfig.Type,
		DBName:         h.databaseConfig.Name,
	}

	if total, err := h.userService.CountAll(ctx); err == nil {
		info.TotalUsers = total
	}

	if _, total, err := h.musicService.Search(ctx, &domain.MusicSearchParams{Page: 1, PageSize: 1}, nil); err == nil {
		info.TotalMusic = total
	}

	if total, err := h.musicService.CountWithMetadata(ctx); err == nil {
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
	page, pageSize := parsePagination(c, 10)

	users, total, err := h.userService.SearchUsers(c.Request.Context(), query, page, pageSize)
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

func (h *AdminHandler) CreateUser(c *gin.Context) {
	var req domain.AdminCreateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, "Invalid request parameters")
		return
	}
	user, err := h.userService.CreateUserByAdmin(c.Request.Context(), &req)
	if err != nil {
		switch {
		case errors.Is(err, password.ErrPasswordTooShort), errors.Is(err, password.ErrPasswordTooLong):
			BadRequest(c, "Password must be at least 8 Unicode characters and no more than 72 UTF-8 bytes")
		case errors.Is(err, repository.ErrUsernameExists):
			Error(c, http.StatusConflict, "Username already exists")
		case errors.Is(err, repository.ErrEmailExists):
			Error(c, http.StatusConflict, "Email already exists")
		default:
			InternalServerError(c, "Failed to create user")
		}
		return
	}
	Created(c, user)
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

	currentUserID := c.GetUint("userID")
	if uint(id) == currentUserID {
		BadRequest(c, "Cannot modify your own account")
		return
	}

	// If disabling an admin, ensure they're not the last active admin
	if !req.IsActive {
		targetUser, err := h.userService.GetUserByID(c.Request.Context(), uint(id))
		if err != nil {
			InternalServerError(c, "Failed to fetch user")
			return
		}
		if targetUser.Role == "admin" {
			adminCount, err := h.userService.CountAdmins(c.Request.Context())
			if err != nil {
				InternalServerError(c, "Failed to count admins")
				return
			}
			if adminCount <= 1 {
				BadRequest(c, "Cannot disable the last active admin")
				return
			}
		}
	}

	if err := h.userService.UpdateUserStatus(c.Request.Context(), uint(id), req.IsActive); err != nil {
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

	currentUserID := c.GetUint("userID")
	if uint(id) == currentUserID {
		BadRequest(c, "Cannot modify your own account")
		return
	}

	// If changing from admin to non-admin, ensure they're not the last active admin
	if req.Role != "admin" {
		targetUser, err := h.userService.GetUserByID(c.Request.Context(), uint(id))
		if err != nil {
			InternalServerError(c, "Failed to fetch user")
			return
		}
		if targetUser.Role == "admin" {
			adminCount, err := h.userService.CountAdmins(c.Request.Context())
			if err != nil {
				InternalServerError(c, "Failed to count admins")
				return
			}
			if adminCount <= 1 {
				BadRequest(c, "Cannot remove the last active admin role")
				return
			}
		}
	}

	if err := h.userService.UpdateUserRole(c.Request.Context(), uint(id), req.Role); err != nil {
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

	if err := h.musicService.AdminDelete(c.Request.Context(), uint(id)); err != nil {
		if errors.Is(err, repository.ErrMusicNotFound) {
			NotFound(c, "Music not found")
		} else {
			InternalServerError(c, "Failed to delete music")
		}
		return
	}

	Success(c, nil)
}

func (h *AdminHandler) ListMediaLibraryRoots(c *gin.Context) {
	roots, err := h.mediaLibraryService.ListRoots(c.Request.Context())
	if err != nil {
		InternalServerError(c, "Failed to fetch media library roots")
		return
	}
	Success(c, roots)
}

func (h *AdminHandler) CreateMediaLibraryRoot(c *gin.Context) {
	var req domain.CreateMediaLibraryRootRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, "Invalid media library root")
		return
	}
	root, err := h.mediaLibraryService.CreateRoot(c.Request.Context(), c.GetUint("userID"), &req)
	if err != nil {
		handleMediaLibraryError(c, err, "Failed to create media library root")
		return
	}
	Created(c, root)
}

func (h *AdminHandler) UpdateMediaLibraryRoot(c *gin.Context) {
	id, ok := parseMediaLibraryID(c, "Invalid media library root ID")
	if !ok {
		return
	}
	var req domain.UpdateMediaLibraryRootRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, "Invalid media library root")
		return
	}
	root, err := h.mediaLibraryService.UpdateRoot(c.Request.Context(), id, &req)
	if err != nil {
		handleMediaLibraryError(c, err, "Failed to update media library root")
		return
	}
	Success(c, root)
}

func (h *AdminHandler) DeleteMediaLibraryRoot(c *gin.Context) {
	id, ok := parseMediaLibraryID(c, "Invalid media library root ID")
	if !ok {
		return
	}
	if err := h.mediaLibraryService.DeleteRoot(c.Request.Context(), id); err != nil {
		handleMediaLibraryError(c, err, "Failed to delete media library root")
		return
	}
	Success(c, nil)
}

func (h *AdminHandler) ProbeMediaLibraryRoot(c *gin.Context) {
	id, ok := parseMediaLibraryID(c, "Invalid media library root ID")
	if !ok {
		return
	}
	health, err := h.mediaLibraryService.ProbeRoot(c.Request.Context(), id)
	if err != nil {
		handleMediaLibraryError(c, err, "Failed to probe media library root")
		return
	}
	Success(c, health)
}

func (h *AdminHandler) StartMediaLibraryScan(c *gin.Context) {
	rootID, ok := parseMediaLibraryID(c, "Invalid media library root ID")
	if !ok {
		return
	}
	job, err := h.mediaLibraryService.StartScan(c.Request.Context(), rootID, c.GetUint("userID"))
	if err != nil {
		handleMediaLibraryError(c, err, "Failed to start media library scan")
		return
	}
	Created(c, job)
}

func (h *AdminHandler) ListMediaLibraryScans(c *gin.Context) {
	var rootID *uint
	if value := c.Query("root_id"); value != "" {
		parsed, err := strconv.ParseUint(value, 10, 32)
		if err != nil {
			BadRequest(c, "Invalid media library root ID")
			return
		}
		id := uint(parsed)
		rootID = &id
	}
	page, pageSize := parsePagination(c, 20)
	jobs, total, err := h.mediaLibraryService.ListScanJobs(c.Request.Context(), rootID, page, pageSize)
	if err != nil {
		InternalServerError(c, "Failed to fetch media library scans")
		return
	}
	Success(c, gin.H{"items": jobs, "total": total, "page": page, "size": pageSize})
}

func (h *AdminHandler) GetMediaLibraryScan(c *gin.Context) {
	id, ok := parseMediaLibraryID(c, "Invalid media library scan ID")
	if !ok || id == 0 {
		if ok {
			BadRequest(c, "Invalid media library scan ID")
		}
		return
	}
	job, err := h.mediaLibraryService.GetScanJob(c.Request.Context(), id)
	if err != nil {
		handleMediaLibraryError(c, err, "Failed to fetch media library scan")
		return
	}
	Success(c, job)
}

func (h *AdminHandler) CancelMediaLibraryScan(c *gin.Context) {
	id, ok := parseMediaLibraryID(c, "Invalid media library scan ID")
	if !ok || id == 0 {
		if ok {
			BadRequest(c, "Invalid media library scan ID")
		}
		return
	}
	job, err := h.mediaLibraryService.CancelScan(c.Request.Context(), id)
	if err != nil {
		handleMediaLibraryError(c, err, "Failed to cancel media library scan")
		return
	}
	Success(c, job)
}

func parseMediaLibraryID(c *gin.Context, message string) (uint, bool) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		BadRequest(c, message)
		return 0, false
	}
	return uint(id), true
}

func handleMediaLibraryError(c *gin.Context, err error, fallback string) {
	switch {
	case errors.Is(err, repository.ErrMediaRootNotFound), errors.Is(err, repository.ErrMediaScanNotFound):
		NotFound(c, "Media library resource not found")
	case errors.Is(err, repository.ErrMediaRootInUse), errors.Is(err, repository.ErrMediaScanInProgress),
		errors.Is(err, service.ErrMediaRootOverlap), errors.Is(err, service.ErrMediaRootPathLocked):
		Error(c, http.StatusConflict, err.Error())
	case errors.Is(err, service.ErrInvalidMediaRoot):
		BadRequest(c, err.Error())
	case errors.Is(err, service.ErrLibraryScannerDisabled):
		Error(c, http.StatusServiceUnavailable, "Media library scanner is disabled")
	default:
		InternalServerError(c, fallback)
	}
}
