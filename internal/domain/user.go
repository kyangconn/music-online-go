package domain

import (
	"time"

	"gorm.io/gorm"
)

type User struct {
	ID        uint           `json:"id" gorm:"primarykey"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `json:"-" gorm:"index"`

	// 基础信息
	Username string `json:"username" gorm:"uniqueIndex;size:100;not null"`
	Email    string `json:"email" gorm:"uniqueIndex;size:255;not null"`
	Password string `json:"-" gorm:"size:255;not null"` // 不返回给客户端

	// 个人信息
	FullName  string `json:"full_name" gorm:"size:255"`
	Nickname  string `json:"nickname" gorm:"size:100"`
	AvatarURL string `json:"avatar_url" gorm:"size:500"`
	Phone     string `json:"phone" gorm:"size:20"`
	Bio       string `json:"bio" gorm:"type:text"`

	// 状态
	IsActive   bool `json:"is_active" gorm:"default:true"`
	IsVerified bool `json:"is_verified" gorm:"default:false"`

	// 角色权限（简单实现，复杂场景可以用RBAC）
	Role string `json:"role" gorm:"size:50;default:'user'"`
}

// 注册请求DTO
type RegisterRequest struct {
	Username string `json:"username" binding:"required,min=3,max=50"`
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=6,max=100"`
	FullName string `json:"full_name" binding:"max=255"`
}

// 登录请求DTO
type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// 更新用户请求DTO
type UpdateUserRequest struct {
	FullName  *string `json:"full_name"`
	Nickname  *string `json:"nickname"`
	AvatarURL *string `json:"avatar_url"`
	Phone     *string `json:"phone"`
	Bio       *string `json:"bio"`
	Email     *string `json:"email" binding:"omitempty,email"`
}

// 用户响应DTO（过滤敏感信息）
type UserResponse struct {
	ID         uint      `json:"id"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
	Username   string    `json:"username"`
	Email      string    `json:"email"`
	FullName   string    `json:"full_name"`
	Nickname   string    `json:"nickname"`
	AvatarURL  string    `json:"avatar_url"`
	Phone      string    `json:"phone"`
	Bio        string    `json:"bio"`
	IsActive   bool      `json:"is_active"`
	IsVerified bool      `json:"is_verified"`
	Role       string    `json:"role"`
}

// 登录响应DTO
type LoginResponse struct {
	User  *UserResponse `json:"user"`
	Token string        `json:"token"`
}

// 转换为响应DTO
func (u *User) ToResponse() *UserResponse {
	if u == nil {
		return nil
	}
	return &UserResponse{
		ID:         u.ID,
		CreatedAt:  u.CreatedAt,
		UpdatedAt:  u.UpdatedAt,
		Username:   u.Username,
		Email:      u.Email,
		FullName:   u.FullName,
		Nickname:   u.Nickname,
		AvatarURL:  u.AvatarURL,
		Phone:      u.Phone,
		Bio:        u.Bio,
		IsActive:   u.IsActive,
		IsVerified: u.IsVerified,
		Role:       u.Role,
	}
}
