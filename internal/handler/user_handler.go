// Package handler user_handler.go - 用户处理器
// 处理用户相关的 HTTP 请求：注册、登录、资料管理、TOTP 设置
package handler

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/kyangconn/music-online-go/internal/domain"
	"github.com/kyangconn/music-online-go/internal/pkg/password"
	"github.com/kyangconn/music-online-go/internal/repository"
	"github.com/kyangconn/music-online-go/internal/service"
)

// UserHandler handles HTTP requests related to user operations.
type UserHandler struct {
	userService service.UserService
}

func NewUserHandler(userService service.UserService) *UserHandler {
	return &UserHandler{userService: userService}
}

// Register godoc
// @Summary 用户注册
// @Description 注册新用户
// @Tags users
// @Accept json
// @Produce json
// @Param request body domain.RegisterRequest true "注册信息"
// @Success 201 {object} Response "注册成功"
// @Failure 400 {object} Response "请求参数错误"
// @Failure 409 {object} Response "用户名或邮箱已存在"
// @Failure 500 {object} Response "服务器内部错误"
// @Router /api/v1/users/register [post]
func (h *UserHandler) Register(c *gin.Context) {
	var req domain.RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, "Invalid request parameters")
		return
	}

	user, err := h.userService.Register(c.Request.Context(), &req)
	if err != nil {
		switch {
		case errors.Is(err, password.ErrPasswordTooShort), errors.Is(err, password.ErrPasswordTooLong):
			BadRequest(c, "Password must be at least 8 Unicode characters and no more than 72 UTF-8 bytes")
		case errors.Is(err, repository.ErrUsernameExists):
			Error(c, http.StatusConflict, "Username already exists")
		case errors.Is(err, repository.ErrEmailExists):
			Error(c, http.StatusConflict, "Email already exists")
		default:
			InternalServerError(c, "Failed to register user")
		}
		return
	}

	Created(c, user)
}

// Login godoc
// @Summary 用户登录
// @Description 用户登录获取令牌
// @Tags users
// @Accept json
// @Produce json
// @Param request body domain.LoginRequest true "登录信息"
// @Success 200 {object} Response "登录成功"
// @Failure 400 {object} Response "请求参数错误"
// @Failure 401 {object} Response "用户名或密码错误"
// @Failure 403 {object} Response "账户未激活"
// @Router /api/v1/users/login [post]
func (h *UserHandler) Login(c *gin.Context) {
	var req domain.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, "Invalid request parameters")
		return
	}

	response, err := h.userService.Login(c.Request.Context(), &req)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidCredentials):
			Unauthorized(c, "Invalid username or password")
		case errors.Is(err, service.ErrTOTPCodeRequired):
			Unauthorized(c, "TOTP code required")
		case errors.Is(err, service.ErrInvalidTOTPCode):
			Unauthorized(c, "Invalid TOTP code")
		case errors.Is(err, service.ErrAccountInactive):
			Forbidden(c, "Account is inactive")
		default:
			InternalServerError(c, "Failed to login")
		}
		return
	}

	Success(c, response)
}

// GetUserProfile godoc
// @Summary 获取用户资料
// @Description 获取当前登录用户的资料
// @Tags users
// @Accept json
// @Produce json
// @Security BearerAuth
// @Success 200 {object} Response "获取成功"
// @Failure 401 {object} Response "未授权"
// @Failure 404 {object} Response "用户不存在"
// @Router /api/v1/users/profile [get]
func (h *UserHandler) GetUserProfile(c *gin.Context) {
	userID := c.GetUint("userID")

	user, err := h.userService.GetUserByID(c.Request.Context(), userID)
	if err != nil {
		NotFound(c, "User not found")
		return
	}

	Success(c, user)
}

// GetUserByID godoc
// @Summary 获取用户信息
// @Description 根据用户ID获取用户信息
// @Tags users
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "用户ID"
// @Success 200 {object} Response "获取成功"
// @Failure 401 {object} Response "未授权"
// @Failure 404 {object} Response "用户不存在"
// @Router /api/v1/users/{id} [get]
func (h *UserHandler) GetUserByID(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		BadRequest(c, "Invalid user ID")
		return
	}

	user, err := h.userService.GetUserByID(c.Request.Context(), uint(id))
	if err != nil {
		NotFound(c, "User not found")
		return
	}

	Success(c, user)
}

// UpdateUser godoc
// @Summary 更新用户信息
// @Description 更新当前登录用户的信息
// @Tags users
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body domain.UpdateUserRequest true "更新信息"
// @Success 200 {object} Response "更新成功"
// @Failure 400 {object} Response "请求参数错误"
// @Failure 401 {object} Response "未授权"
// @Failure 409 {object} Response "邮箱已被使用"
// @Router /api/v1/users/profile [put]
func (h *UserHandler) UpdateUser(c *gin.Context) {
	userID := c.GetUint("userID")

	var req domain.UpdateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, "Invalid request parameters")
		return
	}

	user, err := h.userService.UpdateUser(c.Request.Context(), userID, &req)
	if err != nil {
		switch {
		case errors.Is(err, repository.ErrEmailExists):
			Error(c, http.StatusConflict, "Email already exists")
		default:
			InternalServerError(c, "Failed to update user")
		}
		return
	}

	Success(c, user)
}

// DeleteUser godoc
// @Summary 删除用户
// @Description 删除当前登录用户的账户
// @Tags users
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body domain.DeleteAccountRequest true "当前密码"
// @Success 200 {object} Response "删除成功"
// @Failure 401 {object} Response "未授权"
// @Failure 404 {object} Response "用户不存在"
// @Router /api/v1/users/profile [delete]
func (h *UserHandler) DeleteUser(c *gin.Context) {
	userID := c.GetUint("userID")

	var req domain.DeleteAccountRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, "Current password is required")
		return
	}

	if err := h.userService.DeleteUser(c.Request.Context(), userID, req.Password); err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidCredentials):
			BadRequest(c, "Current password is incorrect")
		case errors.Is(err, service.ErrLastActiveAdmin):
			BadRequest(c, "Cannot delete the last active admin")
		case errors.Is(err, repository.ErrUserNotFound):
			NotFound(c, "User not found")
		default:
			InternalServerError(c, "Failed to delete account")
		}
		return
	}

	Success(c, gin.H{"message": "User deleted successfully"})
}

// ChangePassword godoc
// @Summary 修改密码
// @Description 修改当前登录用户的密码
// @Tags users
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body ChangePasswordRequest true "密码信息"
// @Success 200 {object} Response "修改成功"
// @Failure 400 {object} Response "请求参数错误"
// @Failure 400 {object} Response "旧密码错误"
// @Router /api/v1/users/change-password [post]
func (h *UserHandler) ChangePassword(c *gin.Context) {
	userID := c.GetUint("userID")

	var req ChangePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, "Invalid request parameters")
		return
	}

	if err := h.userService.ChangePassword(c.Request.Context(), userID, req.OldPassword, req.NewPassword); err != nil {
		switch {
		case errors.Is(err, service.ErrOldPasswordIncorrect):
			BadRequest(c, "Old password is incorrect")
		case errors.Is(err, password.ErrPasswordTooShort), errors.Is(err, password.ErrPasswordTooLong):
			BadRequest(c, "Password must be at least 8 Unicode characters and no more than 72 UTF-8 bytes")
		default:
			InternalServerError(c, "Failed to change password")
		}
		return
	}

	Success(c, gin.H{"message": "Password changed successfully"})
}

// SetupTOTP godoc
// @Summary 设置TOTP
// @Tags users
// @Accept json
// @Produce json
// @Security BearerAuth
// @Success 200 {object} domain.TOTPSetupResponse
// @Router /api/v1/users/totp/setup [post]
func (h *UserHandler) SetupTOTP(c *gin.Context) {
	userID := c.GetUint("userID")

	resp, err := h.userService.SetupTOTP(c.Request.Context(), userID)
	if err != nil {
		if errors.Is(err, service.ErrTOTPAlreadyEnabled) {
			Error(c, http.StatusConflict, "TOTP is already enabled")
			return
		}
		InternalServerError(c, "Failed to setup TOTP")
		return
	}

	Created(c, resp)
}

// EnableTOTP godoc
// @Summary 启用TOTP
// @Tags users
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body domain.TOTPVerifyRequest true "验证码"
// @Success 200 {object} Response
// @Router /api/v1/users/totp/enable [post]
func (h *UserHandler) EnableTOTP(c *gin.Context) {
	userID := c.GetUint("userID")

	var req domain.TOTPVerifyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, "Invalid request parameters")
		return
	}

	if err := h.userService.EnableTOTP(c.Request.Context(), userID, req.Code); err != nil {
		switch {
		case errors.Is(err, service.ErrTOTPAlreadyEnabled):
			Error(c, http.StatusConflict, "TOTP is already enabled")
		case errors.Is(err, service.ErrTOTPNotSetUp):
			BadRequest(c, "TOTP not set up yet")
		case errors.Is(err, service.ErrInvalidTOTPCode):
			Unauthorized(c, "Invalid TOTP code")
		default:
			InternalServerError(c, "Failed to enable TOTP")
		}
		return
	}

	Success(c, gin.H{"message": "TOTP enabled successfully"})
}

// DisableTOTP godoc
// @Summary 禁用TOTP
// @Tags users
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body domain.TOTPDisableRequest true "验证码"
// @Success 200 {object} Response
// @Router /api/v1/users/totp/disable [post]
func (h *UserHandler) DisableTOTP(c *gin.Context) {
	userID := c.GetUint("userID")

	var req domain.TOTPDisableRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, "Invalid request parameters")
		return
	}

	if err := h.userService.DisableTOTP(c.Request.Context(), userID, req.Code); err != nil {
		switch {
		case errors.Is(err, service.ErrTOTPNotEnabled):
			BadRequest(c, "TOTP is not enabled")
		case errors.Is(err, service.ErrInvalidTOTPCode):
			Unauthorized(c, "Invalid TOTP code")
		default:
			InternalServerError(c, "Failed to disable TOTP")
		}
		return
	}

	Success(c, gin.H{"message": "TOTP disabled successfully"})
}

// ListUsers godoc
// @Summary 用户列表
// @Description 获取用户列表（分页）
// @Tags users
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param page query int false "页码" default(1)
// @Param page_size query int false "每页数量" default(20)
// @Success 200 {object} Response "获取成功"
// @Failure 401 {object} Response "未授权"
// @Router /api/v1/users [get]
func (h *UserHandler) ListUsers(c *gin.Context) {
	page, pageSize := parsePagination(c, 20)

	users, total, err := h.userService.ListUsers(c.Request.Context(), page, pageSize)
	if err != nil {
		InternalServerError(c, "Failed to list users")
		return
	}

	Success(c, gin.H{
		"users": users,
		"total": total,
		"page":  page,
		"size":  pageSize,
	})
}

type ChangePasswordRequest struct {
	OldPassword string `json:"old_password" binding:"required"`
	NewPassword string `json:"new_password" binding:"required"`
}
