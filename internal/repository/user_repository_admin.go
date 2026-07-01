// Package repository user_repository_admin.go
// 该文件包含用户管理相关的数据库操作，包括用户状态更新、角色更新和搜索功能
package repository

import (
	"context"

	"github.com/kyangconn/music-online-go/internal/domain"
)

// updateUserField 通用用户字段更新函数
// 减少重复代码，统一处理用户字段更新操作
func (r *userRepository) updateUserField(ctx context.Context, id uint, field string, value interface{}) error {
	result := r.db.WithContext(ctx).Model(&domain.User{}).Where("id = ?", id).Update(field, value)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrUserNotFound
	}
	return nil
}

// UpdateStatus 更新用户状态（激活/禁用）
func (r *userRepository) UpdateStatus(ctx context.Context, id uint, isActive bool) error {
	return r.updateUserField(ctx, id, "is_active", isActive)
}

// UpdateRole 更新用户角色
func (r *userRepository) UpdateRole(ctx context.Context, id uint, role string) error {
	return r.updateUserField(ctx, id, "role", role)
}

// Search 搜索用户
// 支持按用户名、邮箱或全名进行模糊搜索，并支持分页
func (r *userRepository) Search(ctx context.Context, query string, page, pageSize int) ([]*domain.User, int64, error) {
	var users []*domain.User
	var total int64
	offset := (page - 1) * pageSize

	db := r.db.WithContext(ctx).Model(&domain.User{})
	if query != "" {
		likeQuery := "%" + query + "%"
		db = db.Where("username LIKE ? OR email LIKE ? OR full_name LIKE ?", likeQuery, likeQuery, likeQuery)
	}

	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	if err := db.Offset(offset).Limit(pageSize).Find(&users).Error; err != nil {
		return nil, 0, err
	}

	return users, total, nil
}

// SetTOTPSecret 设置用户的TOTP密钥
func (r *userRepository) SetTOTPSecret(ctx context.Context, id uint, secret string) error {
	return r.updateUserField(ctx, id, "totp_secret", secret)
}

// SetTOTPEnabled 设置用户的TOTP启用状态
func (r *userRepository) SetTOTPEnabled(ctx context.Context, id uint, enabled bool) error {
	return r.updateUserField(ctx, id, "totp_enabled", enabled)
}
