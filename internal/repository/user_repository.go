// Package repository user_repository.go - 用户仓库层
// 用户实体的增删改查、存在性检查
package repository

import (
	"context"
	"errors"

	"gorm.io/gorm"

	"github.com/kyangconn/music-online-go/internal/domain"
)

var (
	ErrUserNotFound   = errors.New("user not found")
	ErrUsernameExists = errors.New("username already exists")
	ErrEmailExists    = errors.New("email already exists")
)

// UserRepository defines user data access operations.
type UserRepository interface {
	Create(ctx context.Context, user *domain.User) error
	FindByID(ctx context.Context, id uint) (*domain.User, error)
	FindByUsername(ctx context.Context, username string) (*domain.User, error)
	FindByEmail(ctx context.Context, email string) (*domain.User, error)
	Update(ctx context.Context, user *domain.User) error
	Delete(ctx context.Context, id uint) error
	ExistsByUsername(ctx context.Context, username string) (bool, error)
	ExistsByEmail(ctx context.Context, email string) (bool, error)
	List(ctx context.Context, page, pageSize int) ([]*domain.User, int64, error)
	// Admin specific methods
	UpdateStatus(ctx context.Context, id uint, isActive bool) error
	UpdateRole(ctx context.Context, id uint, role string) error
	Search(ctx context.Context, query string, page, pageSize int) ([]*domain.User, int64, error)
	// 统计
	CountAll(ctx context.Context) (int64, error)
	// TOTP
	SetTOTPSecret(ctx context.Context, id uint, secret string) error
	SetTOTPEnabled(ctx context.Context, id uint, enabled bool) error
}

type userRepository struct {
	db *gorm.DB
}

func NewUserRepository(db *gorm.DB) UserRepository {
	return &userRepository{db: db}
}

func (r *userRepository) Create(ctx context.Context, user *domain.User) error {
	// 检查用户名是否已存在
	exists, err := r.ExistsByUsername(ctx, user.Username)
	if err != nil {
		return err
	}
	if exists {
		return ErrUsernameExists
	}

	// 检查邮箱是否已存在
	exists, err = r.ExistsByEmail(ctx, user.Email)
	if err != nil {
		return err
	}
	if exists {
		return ErrEmailExists
	}

	return r.db.WithContext(ctx).Create(user).Error
}

func (r *userRepository) FindByID(ctx context.Context, id uint) (*domain.User, error) {
	var user domain.User
	if err := r.db.WithContext(ctx).First(&user, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}
	return &user, nil
}

func (r *userRepository) FindByUsername(ctx context.Context, username string) (*domain.User, error) {
	var user domain.User
	if err := r.db.WithContext(ctx).Where("username = ?", username).First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}
	return &user, nil
}

func (r *userRepository) FindByEmail(ctx context.Context, email string) (*domain.User, error) {
	var user domain.User
	if err := r.db.WithContext(ctx).Where("email = ?", email).First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}
	return &user, nil
}

func (r *userRepository) Update(ctx context.Context, user *domain.User) error {
	// 检查邮箱是否被其他用户使用
	if user.Email != "" {
		var existingUser domain.User
		if err := r.db.WithContext(ctx).Where("email = ? AND id != ?", user.Email, user.ID).First(&existingUser).Error; err == nil {
			return ErrEmailExists
		}
	}

	return r.db.WithContext(ctx).Save(user).Error
}

func (r *userRepository) Delete(ctx context.Context, id uint) error {
	result := r.db.WithContext(ctx).Delete(&domain.User{}, id)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrUserNotFound
	}
	return nil
}

func (r *userRepository) ExistsByUsername(ctx context.Context, username string) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&domain.User{}).Where("username = ?", username).Count(&count).Error
	return count > 0, err
}

func (r *userRepository) ExistsByEmail(ctx context.Context, email string) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&domain.User{}).Where("email = ?", email).Count(&count).Error
	return count > 0, err
}

func (r *userRepository) List(ctx context.Context, page, pageSize int) ([]*domain.User, int64, error) {
	var users []*domain.User
	var total int64

	offset := (page - 1) * pageSize

	// 获取总数
	if err := r.db.WithContext(ctx).Model(&domain.User{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// 获取分页数据
	if err := r.db.WithContext(ctx).Offset(offset).Limit(pageSize).Find(&users).Error; err != nil {
		return nil, 0, err
	}

	return users, total, nil
}

// CountAll 统计所有用户数量
func (r *userRepository) CountAll(ctx context.Context) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&domain.User{}).Count(&count).Error
	return count, err
}
