package repository

import (
	"errors"

	"gorm.io/gorm"

	"github.com/kyangconn/music-online-web/internal/domain"
)

var (
	ErrUserNotFound      = errors.New("user not found")
	ErrUsernameExists    = errors.New("username already exists")
	ErrEmailExists       = errors.New("email already exists")
	ErrInvalidUserStatus = errors.New("invalid user status")
)

type UserRepository interface {
	Create(user *domain.User) error
	FindByID(id uint) (*domain.User, error)
	FindByUsername(username string) (*domain.User, error)
	FindByEmail(email string) (*domain.User, error)
	Update(user *domain.User) error
	Delete(id uint) error
	ExistsByUsername(username string) (bool, error)
	ExistsByEmail(email string) (bool, error)
	List(page, pageSize int) ([]*domain.User, int64, error)
	// Admin specific methods
	UpdateStatus(id uint, isActive bool) error
	UpdateRole(id uint, role string) error
	Search(query string, page, pageSize int) ([]*domain.User, int64, error)
}

type userRepository struct {
	db *gorm.DB
}

func NewUserRepository(db *gorm.DB) UserRepository {
	return &userRepository{db: db}
}

func (r *userRepository) Create(user *domain.User) error {
	// 检查用户名是否已存在
	exists, err := r.ExistsByUsername(user.Username)
	if err != nil {
		return err
	}
	if exists {
		return ErrUsernameExists
	}

	// 检查邮箱是否已存在
	exists, err = r.ExistsByEmail(user.Email)
	if err != nil {
		return err
	}
	if exists {
		return ErrEmailExists
	}

	return r.db.Create(user).Error
}

func (r *userRepository) FindByID(id uint) (*domain.User, error) {
	var user domain.User
	if err := r.db.First(&user, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}
	return &user, nil
}

func (r *userRepository) FindByUsername(username string) (*domain.User, error) {
	var user domain.User
	if err := r.db.Where("username = ?", username).First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}
	return &user, nil
}

func (r *userRepository) FindByEmail(email string) (*domain.User, error) {
	var user domain.User
	if err := r.db.Where("email = ?", email).First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}
	return &user, nil
}

func (r *userRepository) Update(user *domain.User) error {
	// 检查邮箱是否被其他用户使用
	if user.Email != "" {
		var existingUser domain.User
		if err := r.db.Where("email = ? AND id != ?", user.Email, user.ID).First(&existingUser).Error; err == nil {
			return ErrEmailExists
		}
	}

	return r.db.Save(user).Error
}

func (r *userRepository) Delete(id uint) error {
	result := r.db.Delete(&domain.User{}, id)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrUserNotFound
	}
	return nil
}

func (r *userRepository) ExistsByUsername(username string) (bool, error) {
	var count int64
	err := r.db.Model(&domain.User{}).Where("username = ?", username).Count(&count).Error
	return count > 0, err
}

func (r *userRepository) ExistsByEmail(email string) (bool, error) {
	var count int64
	err := r.db.Model(&domain.User{}).Where("email = ?", email).Count(&count).Error
	return count > 0, err
}

func (r *userRepository) List(page, pageSize int) ([]*domain.User, int64, error) {
	var users []*domain.User
	var total int64

	offset := (page - 1) * pageSize

	// 获取总数
	if err := r.db.Model(&domain.User{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// 获取分页数据
	if err := r.db.Offset(offset).Limit(pageSize).Find(&users).Error; err != nil {
		return nil, 0, err
	}

	return users, total, nil
}
