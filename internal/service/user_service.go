package service

import (
	"errors"
	"fmt"

	"github.com/kyangconn/music-online-web/internal/domain"
	"github.com/kyangconn/music-online-web/internal/pkg/jwt"
	"github.com/kyangconn/music-online-web/internal/pkg/password"
	"github.com/kyangconn/music-online-web/internal/repository"
)

type UserService interface {
	Register(req *domain.RegisterRequest) (*domain.UserResponse, error)
	Login(req *domain.LoginRequest) (*domain.LoginResponse, error)
	GetUserByID(id uint) (*domain.UserResponse, error)
	GetUserByUsername(username string) (*domain.UserResponse, error)
	UpdateUser(id uint, req *domain.UpdateUserRequest) (*domain.UserResponse, error)
	DeleteUser(id uint) error
	ChangePassword(userID uint, oldPassword, newPassword string) error
	VerifyUser(id uint) error
	ListUsers(page, pageSize int) ([]*domain.UserResponse, int64, error)
}

type userService struct {
	userRepo repository.UserRepository
}

func NewUserService(userRepo repository.UserRepository) UserService {
	return &userService{userRepo: userRepo}
}

func (s *userService) Register(req *domain.RegisterRequest) (*domain.UserResponse, error) {
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

	if err := s.userRepo.Create(user); err != nil {
		return nil, err
	}

	return user.ToResponse(), nil
}

func (s *userService) Login(req *domain.LoginRequest) (*domain.LoginResponse, error) {
	// 查找用户（支持用户名或邮箱登录）
	var user *domain.User
	var err error

	// 尝试按用户名查找
	user, err = s.userRepo.FindByUsername(req.Username)
	if err != nil {
		// 如果按用户名找不到，尝试按邮箱查找
		user, err = s.userRepo.FindByEmail(req.Username)
		if err != nil {
			return nil, errors.New("invalid credentials")
		}
	}

	// 检查用户状态
	if !user.IsActive {
		return nil, errors.New("account is inactive")
	}

	// 验证密码
	valid, err := password.VerifyPassword(req.Password, user.Password)
	if err != nil {
		return nil, fmt.Errorf("failed to verify password: %w", err)
	}
	if !valid {
		return nil, errors.New("invalid credentials")
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

func (s *userService) GetUserByID(id uint) (*domain.UserResponse, error) {
	user, err := s.userRepo.FindByID(id)
	if err != nil {
		return nil, err
	}
	return user.ToResponse(), nil
}

func (s *userService) GetUserByUsername(username string) (*domain.UserResponse, error) {
	user, err := s.userRepo.FindByUsername(username)
	if err != nil {
		return nil, err
	}
	return user.ToResponse(), nil
}

func (s *userService) UpdateUser(id uint, req *domain.UpdateUserRequest) (*domain.UserResponse, error) {
	user, err := s.userRepo.FindByID(id)
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

	if err := s.userRepo.Update(user); err != nil {
		return nil, err
	}

	return user.ToResponse(), nil
}

func (s *userService) DeleteUser(id uint) error {
	return s.userRepo.Delete(id)
}

func (s *userService) ChangePassword(userID uint, oldPassword, newPassword string) error {
	user, err := s.userRepo.FindByID(userID)
	if err != nil {
		return err
	}

	// 验证旧密码
	valid, err := password.VerifyPassword(oldPassword, user.Password)
	if err != nil {
		return fmt.Errorf("failed to verify password: %w", err)
	}
	if !valid {
		return errors.New("old password is incorrect")
	}

	// 哈希新密码
	hashedPassword, err := password.HashPassword(newPassword)
	if err != nil {
		return fmt.Errorf("failed to hash password: %w", err)
	}

	user.Password = hashedPassword
	return s.userRepo.Update(user)
}

func (s *userService) VerifyUser(id uint) error {
	user, err := s.userRepo.FindByID(id)
	if err != nil {
		return err
	}

	user.IsVerified = true
	return s.userRepo.Update(user)
}

func (s *userService) ListUsers(page, pageSize int) ([]*domain.UserResponse, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	users, total, err := s.userRepo.List(page, pageSize)
	if err != nil {
		return nil, 0, err
	}

	responses := make([]*domain.UserResponse, len(users))
	for i, user := range users {
		responses[i] = user.ToResponse()
	}

	return responses, total, nil
}
