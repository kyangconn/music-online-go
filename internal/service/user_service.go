// Package service user_service.go - 用户服务层
// 包含用户注册、登录、信息更新、密码修改及 TOTP 等业务逻辑
package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/kyangconn/music-online-go/internal/domain"
	"github.com/kyangconn/music-online-go/internal/pkg/jwt"
	"github.com/kyangconn/music-online-go/internal/pkg/password"
	"github.com/kyangconn/music-online-go/internal/repository"
	"github.com/pquerna/otp/totp"
)

var (
	ErrInvalidCredentials   = errors.New("invalid credentials")
	ErrAccountInactive      = errors.New("account is inactive")
	ErrTOTPCodeRequired     = errors.New("totp code required")
	ErrInvalidTOTPCode      = errors.New("invalid totp code")
	ErrOldPasswordIncorrect = errors.New("old password is incorrect")
	ErrBootstrapAdminConfig = errors.New("bootstrap admin config is invalid")
	ErrLastActiveAdmin      = errors.New("cannot delete the last active admin")
)

// UserService defines user business logic operations.
type UserService interface {
	Register(ctx context.Context, req *domain.RegisterRequest) (*domain.UserResponse, error)
	Login(ctx context.Context, req *domain.LoginRequest) (*domain.LoginResponse, error)
	GetUserByID(ctx context.Context, id uint) (*domain.UserResponse, error)
	GetUserByUsername(ctx context.Context, username string) (*domain.UserResponse, error)
	UpdateUser(ctx context.Context, id uint, req *domain.UpdateUserRequest) (*domain.UserResponse, error)
	DeleteUser(ctx context.Context, id uint, currentPassword string) error
	ChangePassword(ctx context.Context, userID uint, oldPassword, newPassword string) error
	VerifyUser(ctx context.Context, id uint) error
	ListUsers(ctx context.Context, page, pageSize int) ([]*domain.UserResponse, int64, error)
	// Admin methods
	UpdateUserStatus(ctx context.Context, id uint, isActive bool) error
	UpdateUserRole(ctx context.Context, id uint, role string) error
	SearchUsers(ctx context.Context, query string, page, pageSize int) ([]*domain.UserResponse, int64, error)
	CountAll(ctx context.Context) (int64, error)
	CountAdmins(ctx context.Context) (int64, error)
	// TOTP
	SetupTOTP(ctx context.Context, userID uint) (*domain.TOTPSetupResponse, error)
	EnableTOTP(ctx context.Context, userID uint, code string) error
	DisableTOTP(ctx context.Context, userID uint, code string) error
	BootstrapAdmin(ctx context.Context, req BootstrapAdminRequest) (*domain.UserResponse, bool, error)
}

type userService struct {
	userRepo repository.UserRepository
}

type BootstrapAdminRequest struct {
	Username      string
	Email         string
	Password      string
	FullName      string
	ResetPassword bool
}

// dummyPasswordHash keeps failed login timing closer when the account does not exist.
const dummyPasswordHash = "$2a$10$OLIc7WDuS61Ho.Ezf91LNO9AOgRWT3WbAmBnvG2OrzkqLR9vOCnpC"

func NewUserService(userRepo repository.UserRepository) UserService {
	return &userService{userRepo: userRepo}
}

func (s *userService) Register(ctx context.Context, req *domain.RegisterRequest) (*domain.UserResponse, error) {
	// 哈希密码
	hashedPassword, err := password.HashPassword(req.Password)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	// 创建用户
	user := &domain.User{
		Username: req.Username,
		Email:    req.Email,
		Password: hashedPassword,
		FullName: req.FullName,
		Role:     "user",
	}

	if err := s.userRepo.Create(ctx, user); err != nil {
		return nil, err
	}

	return user.ToResponse(), nil
}

func (s *userService) BootstrapAdmin(ctx context.Context, req BootstrapAdminRequest) (*domain.UserResponse, bool, error) {
	if req.Username == "" || req.Email == "" {
		return nil, false, fmt.Errorf("%w: username and email are required", ErrBootstrapAdminConfig)
	}
	if err := password.ValidateNewPassword(req.Password); err != nil {
		return nil, false, fmt.Errorf("%w: %w", ErrBootstrapAdminConfig, err)
	}
	if req.FullName == "" {
		req.FullName = "Administrator"
	}

	existing, err := s.userRepo.FindByUsername(ctx, req.Username)
	if err != nil {
		if !errors.Is(err, repository.ErrUserNotFound) {
			return nil, false, err
		}

		hashedPassword, hashErr := password.HashPassword(req.Password)
		if hashErr != nil {
			return nil, false, fmt.Errorf("failed to hash bootstrap admin password: %w", hashErr)
		}
		user := &domain.User{
			Username:   req.Username,
			Email:      req.Email,
			Password:   hashedPassword,
			FullName:   req.FullName,
			Role:       "admin",
			IsActive:   true,
			IsVerified: true,
		}
		if createErr := s.userRepo.Create(ctx, user); createErr != nil {
			return nil, false, createErr
		}
		return user.ToResponse(), true, nil
	}

	changed := false
	if existing.Role != "admin" {
		existing.Role = "admin"
		changed = true
	}
	if !existing.IsActive {
		existing.IsActive = true
		changed = true
	}
	if !existing.IsVerified {
		existing.IsVerified = true
		changed = true
	}
	if existing.FullName == "" {
		existing.FullName = req.FullName
		changed = true
	}
	if req.ResetPassword {
		hashedPassword, hashErr := password.HashPassword(req.Password)
		if hashErr != nil {
			return nil, false, fmt.Errorf("failed to hash bootstrap admin password: %w", hashErr)
		}
		existing.Password = hashedPassword
		changed = true
	}

	if changed {
		if err := s.userRepo.Update(ctx, existing); err != nil {
			return nil, false, err
		}
	}
	return existing.ToResponse(), false, nil
}

func (s *userService) Login(ctx context.Context, req *domain.LoginRequest) (*domain.LoginResponse, error) {
	// 查找用户（支持用户名或邮箱登录）
	var user *domain.User
	var err error

	// 尝试按用户名查找
	user, err = s.userRepo.FindByUsername(ctx, req.Username)
	if err != nil {
		// 只有明确"用户名不存在"时才回退到邮箱查询
		if !errors.Is(err, repository.ErrUserNotFound) {
			return nil, fmt.Errorf("failed to find user: %w", err)
		}
		// 如果按用户名找不到，尝试按邮箱查找
		user, err = s.userRepo.FindByEmail(ctx, req.Username)
		if err != nil {
			if _, verifyErr := password.VerifyPassword(req.Password, dummyPasswordHash); verifyErr != nil {
				return nil, fmt.Errorf("failed to verify password: %w", verifyErr)
			}
			return nil, ErrInvalidCredentials
		}
	}

	// 检查用户状态
	if !user.IsActive {
		return nil, ErrAccountInactive
	}

	// 验证密码
	valid, err := password.VerifyPassword(req.Password, user.Password)
	if err != nil {
		return nil, fmt.Errorf("failed to verify password: %w", err)
	}
	if !valid {
		return nil, ErrInvalidCredentials
	}

	if user.TOTPEnabled {
		if req.TOTPCode == "" {
			return nil, ErrTOTPCodeRequired
		}
		if !totp.Validate(req.TOTPCode, user.TOTPSecret) {
			return nil, ErrInvalidTOTPCode
		}
	}

	// 生成JWT令牌
	token, err := jwt.GenerateToken(user.ID, user.Username, user.Role)
	if err != nil {
		return nil, fmt.Errorf("failed to generate token: %w", err)
	}

	return &domain.LoginResponse{
		User:  user.ToResponse(),
		Token: token,
	}, nil
}

func (s *userService) GetUserByID(ctx context.Context, id uint) (*domain.UserResponse, error) {
	user, err := s.userRepo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return user.ToResponse(), nil
}

func (s *userService) GetUserByUsername(ctx context.Context, username string) (*domain.UserResponse, error) {
	user, err := s.userRepo.FindByUsername(ctx, username)
	if err != nil {
		return nil, err
	}
	return user.ToResponse(), nil
}

func (s *userService) UpdateUser(ctx context.Context, id uint, req *domain.UpdateUserRequest) (*domain.UserResponse, error) {
	user, err := s.userRepo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	// 更新允许修改的字段
	if req.FullName != nil {
		user.FullName = *req.FullName
	}
	if req.Nickname != nil {
		user.Nickname = *req.Nickname
	}
	if req.AvatarURL != nil {
		user.AvatarURL = *req.AvatarURL
	}
	if req.Phone != nil {
		user.Phone = *req.Phone
	}
	if req.Bio != nil {
		user.Bio = *req.Bio
	}
	if req.Email != nil {
		user.Email = *req.Email
	}

	if err := s.userRepo.Update(ctx, user); err != nil {
		return nil, err
	}

	return user.ToResponse(), nil
}

func (s *userService) DeleteUser(ctx context.Context, id uint, currentPassword string) error {
	user, err := s.userRepo.FindByID(ctx, id)
	if err != nil {
		return err
	}

	valid, err := password.VerifyPassword(currentPassword, user.Password)
	if err != nil {
		return fmt.Errorf("failed to verify password: %w", err)
	}
	if !valid {
		return ErrInvalidCredentials
	}

	if user.Role == "admin" && user.IsActive {
		adminCount, err := s.userRepo.CountAdmins(ctx)
		if err != nil {
			return err
		}
		if adminCount <= 1 {
			return ErrLastActiveAdmin
		}
	}

	musicIDs, err := s.userRepo.ListOwnedMusicIDs(ctx, id)
	if err != nil {
		return err
	}
	if err := s.userRepo.Delete(ctx, id); err != nil {
		return err
	}
	for _, musicID := range musicIDs {
		cleanupMusicUploadDirectory(musicID)
	}
	return nil
}

func (s *userService) ChangePassword(ctx context.Context, userID uint, oldPassword, newPassword string) error {
	user, err := s.userRepo.FindByID(ctx, userID)
	if err != nil {
		return err
	}

	// 验证旧密码
	valid, err := password.VerifyPassword(oldPassword, user.Password)
	if err != nil {
		return fmt.Errorf("failed to verify password: %w", err)
	}
	if !valid {
		return ErrOldPasswordIncorrect
	}

	// 哈希新密码
	hashedPassword, err := password.HashPassword(newPassword)
	if err != nil {
		return fmt.Errorf("failed to hash password: %w", err)
	}

	user.Password = hashedPassword
	return s.userRepo.Update(ctx, user)
}

func (s *userService) VerifyUser(ctx context.Context, id uint) error {
	user, err := s.userRepo.FindByID(ctx, id)
	if err != nil {
		return err
	}

	user.IsVerified = true
	return s.userRepo.Update(ctx, user)
}

func (s *userService) ListUsers(ctx context.Context, page, pageSize int) ([]*domain.UserResponse, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	users, total, err := s.userRepo.List(ctx, page, pageSize)
	if err != nil {
		return nil, 0, err
	}

	responses := make([]*domain.UserResponse, len(users))
	for i, user := range users {
		responses[i] = user.ToResponse()
	}

	return responses, total, nil
}
