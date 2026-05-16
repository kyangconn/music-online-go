package service

import (
	"fmt"

	"github.com/kyangconn/music-online-go/internal/domain"
	"github.com/pquerna/otp/totp"
)

func (s *userService) UpdateUserStatus(id uint, isActive bool) error {
	return s.userRepo.UpdateStatus(id, isActive)
}

func (s *userService) UpdateUserRole(id uint, role string) error {
	return s.userRepo.UpdateRole(id, role)
}

func (s *userService) SearchUsers(query string, page, pageSize int) ([]*domain.UserResponse, int64, error) {
	users, total, err := s.userRepo.Search(query, page, pageSize)
	if err != nil {
		return nil, 0, err
	}

	var userResponses []*domain.UserResponse
	for _, user := range users {
		userResponses = append(userResponses, user.ToResponse())
	}

	return userResponses, total, nil
}

func (s *userService) CountAll() (int64, error) {
	return s.userRepo.CountAll()
}

func (s *userService) SetupTOTP(userID uint) (*domain.TOTPSetupResponse, error) {
	user, err := s.userRepo.FindByID(userID)
	if err != nil {
		return nil, err
	}

	if user.TOTPEnabled {
		return nil, fmt.Errorf("totp is already enabled")
	}

	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      "music-online-go",
		AccountName: user.Username,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to generate totp key: %w", err)
	}

	if err := s.userRepo.SetTOTPSecret(userID, key.Secret()); err != nil {
		return nil, fmt.Errorf("failed to save totp secret: %w", err)
	}

	return &domain.TOTPSetupResponse{
		Secret:    key.Secret(),
		QRCodeURL: key.URL(),
	}, nil
}

func (s *userService) EnableTOTP(userID uint, code string) error {
	user, err := s.userRepo.FindByID(userID)
	if err != nil {
		return err
	}

	if user.TOTPEnabled {
		return fmt.Errorf("totp is already enabled")
	}

	if user.TOTPSecret == "" {
		return fmt.Errorf("totp not set up yet, call setup first")
	}

	valid := totp.Validate(code, user.TOTPSecret)
	if !valid {
		return fmt.Errorf("invalid totp code")
	}

	return s.userRepo.SetTOTPEnabled(userID, true)
}

func (s *userService) DisableTOTP(userID uint, code string) error {
	user, err := s.userRepo.FindByID(userID)
	if err != nil {
		return err
	}

	if !user.TOTPEnabled {
		return fmt.Errorf("totp is not enabled")
	}

	valid := totp.Validate(code, user.TOTPSecret)
	if !valid {
		return fmt.Errorf("invalid totp code")
	}

	if err := s.userRepo.SetTOTPEnabled(userID, false); err != nil {
		return err
	}
	return s.userRepo.SetTOTPSecret(userID, "")
}
