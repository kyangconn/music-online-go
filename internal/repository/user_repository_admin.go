package repository

import "github.com/kyangconn/music-online-web/internal/domain"

func (r *userRepository) UpdateStatus(id uint, isActive bool) error {
	result := r.db.Model(&domain.User{}).Where("id = ?", id).Update("is_active", isActive)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrUserNotFound
	}
	return nil
}

func (r *userRepository) UpdateRole(id uint, role string) error {
	result := r.db.Model(&domain.User{}).Where("id = ?", id).Update("role", role)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrUserNotFound
	}
	return nil
}

func (r *userRepository) Search(query string, page, pageSize int) ([]*domain.User, int64, error) {
	var users []*domain.User
	var total int64
	offset := (page - 1) * pageSize

	db := r.db.Model(&domain.User{})
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
