// Package service user_service_admin.go - 用户管理服务
// 包含管理员操作：用户状态更新、角色更新、TOTP 管理等
package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/kyangconn/music-online-go/internal/domain"
	"github.com/pquerna/otp/totp"
)

var (
	ErrTOTPAlreadyEnabled = errors.New("totp is already enabled")
	ErrTOTPNotSetUp       = errors.New("totp not set up yet, call setup first")
	ErrTOTPNotEnabled     = errors.New("totp is not enabled")
)

func (s *userService) UpdateUserStatus(ctx context.Context, id uint, isActive bool) error {
	if err := s.userRepo.UpdateStatus(ctx, id, isActive); err != nil {
		return err
	}
	if !isActive {
		// 禁用账户时立即失效其所有会话。
		return s.sessionRepo.RevokeAllForUser(ctx, id, time.Now())
	}
	return nil
}

func (s *userService) UpdateUserRole(ctx context.Context, id uint, role string) error {
	return s.userRepo.UpdateRole(ctx, id, role)
}

func (s *userService) SearchUsers(ctx context.Context, query string, page, pageSize int) ([]*domain.UserResponse, int64, error) {
	users, total, err := s.userRepo.Search(ctx, query, page, pageSize)
	if err != nil {
		return nil, 0, err
	}

	var userResponses []*domain.UserResponse
	for _, user := range users {
		userResponses = append(userResponses, user.ToResponse())
	}

	return userResponses, total, nil
}

func (s *userService) CountAll(ctx context.Context) (int64, error) {
	return s.userRepo.CountAll(ctx)
}

func (s *userService) CountAdmins(ctx context.Context) (int64, error) {
	return s.userRepo.CountAdmins(ctx)
}

func (s *userService) SetupTOTP(ctx context.Context, userID uint) (*domain.TOTPSetupResponse, error) {
	user, err := s.userRepo.FindByID(ctx, userID)
	if err != nil {
		return nil, err
	}

	if user.TOTPEnabled {
		return nil, ErrTOTPAlreadyEnabled
	}

	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      "music-online-go",
		AccountName: user.Username,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to generate totp key: %w", err)
	}

	if err := s.userRepo.SetTOTPSecret(ctx, userID, key.Secret()); err != nil {
		return nil, fmt.Errorf("failed to save totp secret: %w", err)
	}

	return &domain.TOTPSetupResponse{
		Secret:    key.Secret(),
		QRCodeURL: key.URL(),
	}, nil
}

func (s *userService) EnableTOTP(ctx context.Context, userID uint, code string) error {
	user, err := s.userRepo.FindByID(ctx, userID)
	if err != nil {
		return err
	}

	if user.TOTPEnabled {
		return ErrTOTPAlreadyEnabled
	}

	if user.TOTPSecret == "" {
		return ErrTOTPNotSetUp
	}

	valid := totp.Validate(code, user.TOTPSecret)
	if !valid {
		return ErrInvalidTOTPCode
	}

	return s.userRepo.SetTOTPEnabled(ctx, userID, true)
}

func (s *userService) DisableTOTP(ctx context.Context, userID uint, code string) error {
	user, err := s.userRepo.FindByID(ctx, userID)
	if err != nil {
		return err
	}

	if !user.TOTPEnabled {
		return ErrTOTPNotEnabled
	}

	valid := totp.Validate(code, user.TOTPSecret)
	if !valid {
		return ErrInvalidTOTPCode
	}

	if err := s.userRepo.SetTOTPEnabled(ctx, userID, false); err != nil {
		return err
	}
	return s.userRepo.SetTOTPSecret(ctx, userID, "")
}
