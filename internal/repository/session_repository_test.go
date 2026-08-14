// Package repository session_repository_test.go - 会话数据访问测试
//
// 重点覆盖 RotateIfMatch 的原子条件更新语义：哈希不匹配或已撤销时更新
// 0 行；并发轮换时恰好一个调用成功。SQLite 连接池固定为 1 使并发 UPDATE
// 串行执行，仍然验证条件更新的原子性。
package repository

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"github.com/kyangconn/music-online-go/internal/domain"
)

func openSessionTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := filepath.Join(t.TempDir(), "sessions.db") + "?_pragma=foreign_keys(1)"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open session database: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get session database handle: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() {
		if err := sqlDB.Close(); err != nil {
			t.Errorf("close session database: %v", err)
		}
	})
	if err := db.AutoMigrate(&domain.Session{}); err != nil {
		t.Fatalf("migrate sessions: %v", err)
	}
	return db
}

func createTestSession(t *testing.T, db *gorm.DB, hash string) domain.Session {
	t.Helper()
	session := domain.Session{
		UserID:      7,
		RefreshHash: hash,
		LastUsedAt:  time.Now(),
		ExpiresAt:   time.Now().Add(time.Hour),
	}
	if err := db.Create(&session).Error; err != nil {
		t.Fatalf("create session: %v", err)
	}
	return session
}

func TestSessionRotateIfMatchSwapsHashAndMetadata(t *testing.T) {
	db := openSessionTestDB(t)
	repo := NewSessionRepository(db)
	ctx := context.Background()
	session := createTestSession(t, db, "old-hash")

	now := time.Now()
	rotated, err := repo.RotateIfMatch(ctx, session.ID, "old-hash", "new-hash", "agent/1.0", "127.0.0.1", now)
	if err != nil {
		t.Fatalf("rotate: %v", err)
	}
	if rotated != 1 {
		t.Fatalf("rotated rows = %d, want 1", rotated)
	}

	loaded, err := repo.FindByID(ctx, session.ID)
	if err != nil {
		t.Fatalf("reload session: %v", err)
	}
	if loaded.RefreshHash != "new-hash" {
		t.Fatalf("refresh hash = %q, want new-hash", loaded.RefreshHash)
	}
	if loaded.UserAgent != "agent/1.0" || loaded.IPAddress != "127.0.0.1" {
		t.Fatalf("metadata not updated: %+v", loaded)
	}
	if !loaded.LastUsedAt.Equal(now) {
		t.Fatalf("last_used_at = %v, want %v", loaded.LastUsedAt, now)
	}
}

func TestSessionRotateIfMatchRejectsStaleOrRevoked(t *testing.T) {
	db := openSessionTestDB(t)
	repo := NewSessionRepository(db)
	ctx := context.Background()

	t.Run("wrong hash", func(t *testing.T) {
		session := createTestSession(t, db, "current-hash")
		rotated, err := repo.RotateIfMatch(ctx, session.ID, "stale-hash", "next-hash", "", "", time.Now())
		if err != nil {
			t.Fatalf("rotate with stale hash: %v", err)
		}
		if rotated != 0 {
			t.Fatalf("rotated rows = %d, want 0", rotated)
		}
		loaded, err := repo.FindByID(ctx, session.ID)
		if err != nil {
			t.Fatalf("reload session: %v", err)
		}
		if loaded.RefreshHash != "current-hash" {
			t.Fatalf("hash changed despite stale condition: %q", loaded.RefreshHash)
		}
	})

	t.Run("revoked session", func(t *testing.T) {
		session := createTestSession(t, db, "revoked-hash")
		if err := repo.Revoke(ctx, session.ID, time.Now()); err != nil {
			t.Fatalf("revoke: %v", err)
		}
		rotated, err := repo.RotateIfMatch(ctx, session.ID, "revoked-hash", "next-hash", "", "", time.Now())
		if err != nil {
			t.Fatalf("rotate revoked session: %v", err)
		}
		if rotated != 0 {
			t.Fatalf("rotated rows = %d, want 0", rotated)
		}
	})
}

// TestSessionConcurrentRotationHasSingleWinner 验证两个并发轮换用同一旧哈希
// 时，恰好一个成功，最终哈希必为其中一个新值。
func TestSessionConcurrentRotationHasSingleWinner(t *testing.T) {
	db := openSessionTestDB(t)
	repo := NewSessionRepository(db)
	ctx := context.Background()
	session := createTestSession(t, db, "shared-hash")

	const attempts = 2
	newHashes := []string{"winner-a", "winner-b"}
	results := make([]int64, attempts)
	var wg sync.WaitGroup
	for i := 0; i < attempts; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			rotated, err := repo.RotateIfMatch(ctx, session.ID, "shared-hash", newHashes[index], "", "", time.Now())
			if err != nil {
				t.Errorf("rotate %d: %v", index, err)
				return
			}
			results[index] = rotated
		}(i)
	}
	wg.Wait()

	winners := 0
	for _, rows := range results {
		if rows == 1 {
			winners++
		}
	}
	if winners != 1 {
		t.Fatalf("concurrent rotation winners = %d (results %v), want exactly 1", winners, results)
	}

	loaded, err := repo.FindByID(ctx, session.ID)
	if err != nil {
		t.Fatalf("reload session: %v", err)
	}
	if loaded.RefreshHash != newHashes[0] && loaded.RefreshHash != newHashes[1] {
		t.Fatalf("final hash = %q, want one of %v", loaded.RefreshHash, newHashes)
	}
}

func TestSessionRevokeIsIdempotent(t *testing.T) {
	db := openSessionTestDB(t)
	repo := NewSessionRepository(db)
	ctx := context.Background()
	session := createTestSession(t, db, "to-revoke")

	if err := repo.Revoke(ctx, session.ID, time.Now()); err != nil {
		t.Fatalf("first revoke: %v", err)
	}
	if err := repo.Revoke(ctx, session.ID, time.Now()); err != nil {
		t.Fatalf("second revoke must be a no-op: %v", err)
	}
	loaded, err := repo.FindByID(ctx, session.ID)
	if err != nil {
		t.Fatalf("reload session: %v", err)
	}
	if loaded.RevokedAt == nil {
		t.Fatal("session must be revoked")
	}
}

func TestSessionFindByIDReportsNotFound(t *testing.T) {
	db := openSessionTestDB(t)
	repo := NewSessionRepository(db)

	_, err := repo.FindByID(context.Background(), 424242)
	if !errors.Is(err, ErrSessionNotFound) {
		t.Fatalf("FindByID error = %v, want ErrSessionNotFound", err)
	}
}

func TestSessionDeleteExpiredKeepsActiveRows(t *testing.T) {
	db := openSessionTestDB(t)
	repo := NewSessionRepository(db)
	ctx := context.Background()

	expired := createTestSession(t, db, fmt.Sprintf("expired-%d", time.Now().UnixNano()))
	if err := db.Model(&domain.Session{}).Where("id = ?", expired.ID).Update("expires_at", time.Now().Add(-time.Minute)).Error; err != nil {
		t.Fatalf("backdate expiry: %v", err)
	}
	active := createTestSession(t, db, fmt.Sprintf("active-%d", time.Now().UnixNano()))

	if err := repo.DeleteExpiredForUser(ctx, expired.UserID, time.Now()); err != nil {
		t.Fatalf("delete expired: %v", err)
	}
	if _, err := repo.FindByID(ctx, expired.ID); !errors.Is(err, ErrSessionNotFound) {
		t.Fatalf("expired session should be deleted, got %v", err)
	}
	if _, err := repo.FindByID(ctx, active.ID); err != nil {
		t.Fatalf("active session must survive: %v", err)
	}
}
