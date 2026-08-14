// Package repository session_repository.go - 登录会话数据访问
//
// 会话行承载 refresh token 的生命周期：创建、按哈希查找、原子条件轮换、
// 撤销。轮换通过带旧哈希条件的 UPDATE 保证并发下只有一个请求成功。
package repository

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"

	"github.com/kyangconn/music-online-go/internal/domain"
)

var ErrSessionNotFound = errors.New("session not found")

// SessionRepository defines session data access operations.
type SessionRepository interface {
	Create(ctx context.Context, session *domain.Session) error
	FindByID(ctx context.Context, id uint) (*domain.Session, error)
	// RotateIfMatch atomically replaces the stored hash and refreshes the
	// last-used metadata. It returns the number of rows updated so callers can
	// distinguish a successful rotation from a concurrent one (or a replayed
	// token).
	RotateIfMatch(ctx context.Context, sessionID uint, oldHash, newHash string, userAgent, ipAddress string, now time.Time) (int64, error)
	Revoke(ctx context.Context, id uint, now time.Time) error
	RevokeAllForUser(ctx context.Context, userID uint, now time.Time) error
	RevokeAllExcept(ctx context.Context, userID uint, keepSessionID uint, now time.Time) error
	// DeleteExpiredForUser removes expired sessions to keep the table small.
	DeleteExpiredForUser(ctx context.Context, userID uint, now time.Time) error
	DeleteByID(ctx context.Context, id uint) error
	DeleteByUserID(ctx context.Context, userID uint) error
}

type sessionRepository struct {
	db *gorm.DB
}

func NewSessionRepository(db *gorm.DB) SessionRepository {
	return &sessionRepository{db: db}
}

func (r *sessionRepository) Create(ctx context.Context, session *domain.Session) error {
	return r.db.WithContext(ctx).Create(session).Error
}

func (r *sessionRepository) FindByID(ctx context.Context, id uint) (*domain.Session, error) {
	var session domain.Session
	if err := r.db.WithContext(ctx).First(&session, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrSessionNotFound
		}
		return nil, err
	}
	return &session, nil
}

func (r *sessionRepository) RotateIfMatch(ctx context.Context, sessionID uint, oldHash, newHash string, userAgent, ipAddress string, now time.Time) (int64, error) {
	result := r.db.WithContext(ctx).Model(&domain.Session{}).
		Where("id = ? AND refresh_hash = ? AND revoked_at IS NULL", sessionID, oldHash).
		Updates(map[string]interface{}{
			"refresh_hash": newHash,
			"user_agent":   userAgent,
			"ip_address":   ipAddress,
			"updated_at":   now,
			"last_used_at": now,
		})
	if result.Error != nil {
		return 0, result.Error
	}
	return result.RowsAffected, nil
}

func (r *sessionRepository) Revoke(ctx context.Context, id uint, now time.Time) error {
	return r.db.WithContext(ctx).Model(&domain.Session{}).
		Where("id = ? AND revoked_at IS NULL", id).
		Update("revoked_at", now).Error
}

func (r *sessionRepository) RevokeAllForUser(ctx context.Context, userID uint, now time.Time) error {
	return r.db.WithContext(ctx).Model(&domain.Session{}).
		Where("user_id = ? AND revoked_at IS NULL", userID).
		Update("revoked_at", now).Error
}

func (r *sessionRepository) RevokeAllExcept(ctx context.Context, userID uint, keepSessionID uint, now time.Time) error {
	return r.db.WithContext(ctx).Model(&domain.Session{}).
		Where("user_id = ? AND id != ? AND revoked_at IS NULL", userID, keepSessionID).
		Update("revoked_at", now).Error
}

func (r *sessionRepository) DeleteExpiredForUser(ctx context.Context, userID uint, now time.Time) error {
	return r.db.WithContext(ctx).Where("user_id = ? AND expires_at < ?", userID, now).
		Delete(&domain.Session{}).Error
}

func (r *sessionRepository) DeleteByID(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Where("id = ?", id).Delete(&domain.Session{}).Error
}

func (r *sessionRepository) DeleteByUserID(ctx context.Context, userID uint) error {
	return r.db.WithContext(ctx).Where("user_id = ?", userID).Delete(&domain.Session{}).Error
}
