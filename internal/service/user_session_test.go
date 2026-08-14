// Package service_test user_session_test.go - 可撤销会话生命周期测试
//
// 使用真实 SQLite + 真实 repository 验证：
//   - 登录创建服务端会话并返回可轮换的 refresh token
//   - refresh 轮换后旧 token 立即失效
//   - 宽限窗口内的并发轮换不误杀会话，窗口外的重放触发撤销
//   - 过期/已撤销/禁用账户的会话被拒绝
//   - 登出、全部登出、改密码撤销其他会话
package service_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"github.com/kyangconn/music-online-go/internal/config"
	"github.com/kyangconn/music-online-go/internal/domain"
	"github.com/kyangconn/music-online-go/internal/pkg/password"
	"github.com/kyangconn/music-online-go/internal/repository"
	"github.com/kyangconn/music-online-go/internal/service"
)

// sessionTestEnv bundles a real service wired to an in-memory SQLite database.
type sessionTestEnv struct {
	svc service.UserService
	db  *gorm.DB
	jwt config.JWTConfig
}

func newSessionTestEnv(t *testing.T) sessionTestEnv {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&domain.User{}, &domain.Session{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	jwtCfg := config.JWTConfig{Secret: "0123456789abcdef0123456789abcdef", AccessTokenTTLMinutes: 60, RefreshTokenTTLDays: 30}
	return sessionTestEnv{
		svc: service.NewUserService(repository.NewUserRepository(db), repository.NewSessionRepository(db), t.TempDir(), jwtCfg),
		db:  db,
		jwt: jwtCfg,
	}
}

func (env sessionTestEnv) createUser(t *testing.T, username, rawPassword string) uint {
	t.Helper()
	hash, err := password.HashPassword(rawPassword)
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	user := &domain.User{Username: username, Email: username + "@example.com", Password: hash, IsActive: true}
	if err := repository.NewUserRepository(env.db).Create(context.Background(), user); err != nil {
		t.Fatalf("create user: %v", err)
	}
	return user.ID
}

func (env sessionTestEnv) login(t *testing.T, username, rawPassword string) *domain.LoginResponse {
	t.Helper()
	resp, err := env.svc.Login(context.Background(), &domain.LoginRequest{Username: username, Password: rawPassword})
	if err != nil {
		t.Fatalf("login %s: %v", username, err)
	}
	return resp
}

func TestLoginCreatesServerSideSession(t *testing.T) {
	env := newSessionTestEnv(t)
	userID := env.createUser(t, "session-login", "password123")

	resp := env.login(t, "session-login", "password123")
	if resp.AccessToken == "" || resp.RefreshToken == "" {
		t.Fatal("login must return access and refresh tokens")
	}
	if resp.ExpiresIn != 60*60 {
		t.Fatalf("expires_in = %d, want 3600", resp.ExpiresIn)
	}

	var count int64
	if err := env.db.Model(&domain.Session{}).Where("user_id = ? AND revoked_at IS NULL", userID).Count(&count).Error; err != nil {
		t.Fatalf("count sessions: %v", err)
	}
	if count != 1 {
		t.Fatalf("active sessions = %d, want 1", count)
	}

	// refresh token 只以哈希形式存储，明文不可查询。
	var session domain.Session
	if err := env.db.First(&session).Error; err != nil {
		t.Fatalf("load session: %v", err)
	}
	if session.RefreshHash == resp.RefreshToken || len(session.RefreshHash) != 64 {
		t.Fatalf("refresh token must be stored as a SHA-256 hex hash, got %q", session.RefreshHash)
	}
}

func TestRefreshSessionRotatesTokenAndInvalidatesOldOne(t *testing.T) {
	env := newSessionTestEnv(t)
	env.createUser(t, "rotate-user", "password123")
	login := env.login(t, "rotate-user", "password123")

	refreshed, err := env.svc.RefreshSession(context.Background(), login.RefreshToken, "agent", "127.0.0.1")
	if err != nil {
		t.Fatalf("refresh: %v", err)
	}
	if refreshed.AccessToken == "" || refreshed.RefreshToken == login.RefreshToken {
		t.Fatal("refresh must rotate to a fresh refresh token and mint an access token")
	}

	// 旧 token 再次使用：轮换刚发生（宽限窗口内）→ 并发错误，不撤销会话。
	_, err = env.svc.RefreshSession(context.Background(), login.RefreshToken, "agent", "127.0.0.1")
	if !errors.Is(err, service.ErrConcurrentRefresh) {
		t.Fatalf("replayed old token error = %v, want ErrConcurrentRefresh", err)
	}

	// 新 token 仍然可用。
	again, err := env.svc.RefreshSession(context.Background(), refreshed.RefreshToken, "agent", "127.0.0.1")
	if err != nil {
		t.Fatalf("second refresh: %v", err)
	}
	if again.RefreshToken == refreshed.RefreshToken {
		t.Fatal("second refresh must rotate again")
	}
}

func TestReplayedTokenOutsideGraceWindowRevokesSession(t *testing.T) {
	env := newSessionTestEnv(t)
	env.createUser(t, "reuse-user", "password123")
	login := env.login(t, "reuse-user", "password123")

	// 模拟攻击者持旧 token 迟到的重放：先把数据库中的哈希替换掉（会话已
	// 被轮换过），再把 UpdatedAt 推到宽限窗口之外。UpdatedAt 必须在哈希
	// 之后写，避免 GORM 自动更新时间戳把会话留在窗口内。
	var session domain.Session
	if err := env.db.First(&session).Error; err != nil {
		t.Fatalf("load session: %v", err)
	}
	rotatedHash := "deadbeef" + session.RefreshHash[8:]
	if err := env.db.Model(&domain.Session{}).Where("id = ?", session.ID).Update("refresh_hash", rotatedHash).Error; err != nil {
		t.Fatalf("rotate hash out-of-band: %v", err)
	}
	stale := time.Now().Add(-2 * time.Minute)
	if err := env.db.Model(&domain.Session{}).Where("id = ?", session.ID).Update("updated_at", stale).Error; err != nil {
		t.Fatalf("backdate session: %v", err)
	}

	_, err := env.svc.RefreshSession(context.Background(), login.RefreshToken, "attacker", "10.0.0.9")
	if !errors.Is(err, service.ErrSessionRevoked) {
		t.Fatalf("replay error = %v, want ErrSessionRevoked", err)
	}

	// 会话已被撤销：即使持有新 token 也无法再刷新。
	var revoked domain.Session
	if err := env.db.First(&revoked).Error; err != nil {
		t.Fatalf("reload session: %v", err)
	}
	if revoked.RevokedAt == nil {
		t.Fatal("replayed session must be revoked")
	}
}

func TestRefreshRejectsExpiredRevokedAndInactiveSessions(t *testing.T) {
	env := newSessionTestEnv(t)
	userID := env.createUser(t, "invalid-user", "password123")
	login := env.login(t, "invalid-user", "password123")

	t.Run("unknown token", func(t *testing.T) {
		_, err := env.svc.RefreshSession(context.Background(), "not-a-real-token", "", "")
		if !errors.Is(err, service.ErrInvalidRefreshToken) {
			t.Fatalf("error = %v, want ErrInvalidRefreshToken", err)
		}
	})

	t.Run("revoked session", func(t *testing.T) {
		var session domain.Session
		if err := env.db.First(&session).Error; err != nil {
			t.Fatalf("load session: %v", err)
		}
		if err := env.svc.LogoutSession(context.Background(), userID, session.ID); err != nil {
			t.Fatalf("logout: %v", err)
		}
		_, err := env.svc.RefreshSession(context.Background(), login.RefreshToken, "", "")
		if !errors.Is(err, service.ErrSessionRevoked) {
			t.Fatalf("error = %v, want ErrSessionRevoked", err)
		}
	})

	t.Run("expired session", func(t *testing.T) {
		login2 := env.login(t, "invalid-user", "password123")
		var session domain.Session
		if err := env.db.Where("user_id = ?", userID).Order("id DESC").First(&session).Error; err != nil {
			t.Fatalf("load session: %v", err)
		}
		if err := env.db.Model(&domain.Session{}).Where("id = ?", session.ID).Update("expires_at", time.Now().Add(-time.Minute)).Error; err != nil {
			t.Fatalf("backdate expiry: %v", err)
		}
		_, err := env.svc.RefreshSession(context.Background(), login2.RefreshToken, "", "")
		if !errors.Is(err, service.ErrSessionExpired) {
			t.Fatalf("error = %v, want ErrSessionExpired", err)
		}
	})

	t.Run("inactive account revokes sessions", func(t *testing.T) {
		login3 := env.login(t, "invalid-user", "password123")
		if err := env.svc.UpdateUserStatus(context.Background(), userID, false); err != nil {
			t.Fatalf("disable account: %v", err)
		}
		_, err := env.svc.RefreshSession(context.Background(), login3.RefreshToken, "", "")
		if !errors.Is(err, service.ErrSessionRevoked) {
			t.Fatalf("error = %v, want ErrSessionRevoked", err)
		}
	})
}

func TestLogoutScopesAndLogoutAll(t *testing.T) {
	env := newSessionTestEnv(t)
	userID := env.createUser(t, "logout-user", "password123")
	env.createUser(t, "logout-other", "password123")

	first := env.login(t, "logout-user", "password123")
	second := env.login(t, "logout-user", "password123")
	other := env.login(t, "logout-other", "password123")

	var sessions []domain.Session
	if err := env.db.Order("id").Find(&sessions).Error; err != nil {
		t.Fatalf("list sessions: %v", err)
	}
	if len(sessions) != 3 {
		t.Fatalf("sessions = %d, want 3", len(sessions))
	}

	// 撤销第一个会话（当前设备登出）：只影响该会话。
	if err := env.svc.LogoutSession(context.Background(), userID, sessions[0].ID); err != nil {
		t.Fatalf("logout session: %v", err)
	}
	_, err := env.svc.RefreshSession(context.Background(), first.RefreshToken, "", "")
	if !errors.Is(err, service.ErrSessionRevoked) {
		t.Fatalf("revoked first error = %v, want ErrSessionRevoked", err)
	}
	// 第二个会话不受影响，轮换后必须用新 token 继续验证。
	secondRotated, err := env.svc.RefreshSession(context.Background(), second.RefreshToken, "", "")
	if err != nil {
		t.Fatalf("second session must survive: %v", err)
	}
	// 其他用户的会话不受影响，同样保留轮换后的新 token。
	otherRotated, err := env.svc.RefreshSession(context.Background(), other.RefreshToken, "", "")
	if err != nil {
		t.Fatalf("other user session must survive: %v", err)
	}
	// 撤销不属于该用户的会话 ID 是安全的空操作。
	if err := env.svc.LogoutSession(context.Background(), userID, 9999); err != nil {
		t.Fatalf("foreign logout must be a no-op: %v", err)
	}

	// 全部登出：用户的会话全部失效，其他用户不受影响。
	if err := env.svc.LogoutAllSessions(context.Background(), userID); err != nil {
		t.Fatalf("logout all: %v", err)
	}
	for name, token := range map[string]string{"first": first.RefreshToken, "second": secondRotated.RefreshToken} {
		if _, err := env.svc.RefreshSession(context.Background(), token, "", ""); !errors.Is(err, service.ErrSessionRevoked) {
			t.Fatalf("%s after logout-all error = %v, want ErrSessionRevoked", name, err)
		}
	}
	if _, err := env.svc.RefreshSession(context.Background(), otherRotated.RefreshToken, "", ""); err != nil {
		t.Fatalf("other user must survive logout-all: %v", err)
	}
}

// TestRefreshPropagatesLookupErrors 验证：并发/重放判定时，数据库错误必须
// 向上传播而不是被误判为重放；会话行已消失时安全返回已撤销。
func TestRefreshPropagatesLookupErrors(t *testing.T) {
	user := makeActiveUser(t, "lookup-err", "password123")
	dbErr := errors.New("database connection lost")

	t.Run("database error propagates", func(t *testing.T) {
		userRepo := &stubUserRepo{
			findByIDFn: func(_ context.Context, _ uint) (*domain.User, error) { return user, nil },
		}
		sessionRepo := &stubSessionRepo{
			findByIDFn: func(_ context.Context, _ uint) (*domain.Session, error) {
				return &domain.Session{
					ID:        7,
					UserID:    1,
					ExpiresAt: time.Now().Add(time.Hour),
				}, nil
			},
			rotateFn: func(_ context.Context, _ uint, _, _, _, _ string, _ time.Time) (int64, error) {
				return 0, nil
			},
		}
		// 第二次 FindByID（并发判定时）返回数据库错误。
		callCount := 0
		sessionRepo.findByIDFn = func(_ context.Context, _ uint) (*domain.Session, error) {
			callCount++
			if callCount == 2 {
				return nil, dbErr
			}
			return &domain.Session{ID: 7, UserID: 1, ExpiresAt: time.Now().Add(time.Hour)}, nil
		}
		svc := service.NewUserService(userRepo, sessionRepo, t.TempDir(), testJWTConfig)

		_, err := svc.RefreshSession(context.Background(), "s7.aaaa", "", "")
		if !errors.Is(err, dbErr) {
			t.Fatalf("RefreshSession error = %v, want the underlying DB error", err)
		}
	})

	t.Run("missing session row reports revoked", func(t *testing.T) {
		userRepo := &stubUserRepo{
			findByIDFn: func(_ context.Context, _ uint) (*domain.User, error) { return user, nil },
		}
		sessionRepo := &stubSessionRepo{
			findByIDFn: func(_ context.Context, _ uint) (*domain.Session, error) {
				return &domain.Session{ID: 7, UserID: 1, ExpiresAt: time.Now().Add(time.Hour)}, nil
			},
			rotateFn: func(_ context.Context, _ uint, _, _, _, _ string, _ time.Time) (int64, error) {
				return 0, nil
			},
		}
		callCount := 0
		sessionRepo.findByIDFn = func(_ context.Context, _ uint) (*domain.Session, error) {
			callCount++
			if callCount == 2 {
				return nil, repository.ErrSessionNotFound
			}
			return &domain.Session{ID: 7, UserID: 1, ExpiresAt: time.Now().Add(time.Hour)}, nil
		}
		svc := service.NewUserService(userRepo, sessionRepo, t.TempDir(), testJWTConfig)

		_, err := svc.RefreshSession(context.Background(), "s7.aaaa", "", "")
		if !errors.Is(err, service.ErrSessionRevoked) {
			t.Fatalf("RefreshSession error = %v, want ErrSessionRevoked", err)
		}
	})
}

// TestLoginCleansUpPendingSessionWhenBindingFails 验证：refresh token 绑定
// 失败时，刚创建的占位会话行会被清理，不残留不可用的孤儿数据。
func TestLoginCleansUpPendingSessionWhenBindingFails(t *testing.T) {
	user := makeActiveUser(t, "bind-fail", "password123")
	bindErr := errors.New("database locked")
	deletedID := uint(0)
	userRepo := &stubUserRepo{
		findByUsernameFn: func(_ context.Context, _ string) (*domain.User, error) { return user, nil },
	}
	sessionRepo := &stubSessionRepo{
		createFn: func(_ context.Context, s *domain.Session) error {
			// 模拟真实仓库回填自增 ID。
			s.ID = 99
			return nil
		},
		rotateFn: func(_ context.Context, _ uint, _, _, _, _ string, _ time.Time) (int64, error) {
			return 0, bindErr
		},
		deleteByIDFn: func(_ context.Context, id uint) error {
			deletedID = id
			return nil
		},
	}
	svc := service.NewUserService(userRepo, sessionRepo, t.TempDir(), testJWTConfig)

	_, err := svc.Login(context.Background(), &domain.LoginRequest{Username: "bind-fail", Password: "password123"})
	if !errors.Is(err, bindErr) {
		t.Fatalf("Login error = %v, want the bind error", err)
	}
	if deletedID != 99 {
		t.Fatalf("pending session cleanup deleted id %d, want 99", deletedID)
	}
}

func TestChangePasswordRevokesOtherSessionsButKeepsCurrent(t *testing.T) {
	env := newSessionTestEnv(t)
	env.createUser(t, "password-rotate", "old-pass-123")

	other := env.login(t, "password-rotate", "old-pass-123")
	current := env.login(t, "password-rotate", "old-pass-123")

	var sessions []domain.Session
	if err := env.db.Order("id").Find(&sessions).Error; err != nil {
		t.Fatalf("list sessions: %v", err)
	}
	if len(sessions) != 2 {
		t.Fatalf("sessions = %d, want 2", len(sessions))
	}
	currentSessionID := sessions[1].ID

	if err := env.svc.ChangePassword(context.Background(), sessions[0].UserID, currentSessionID, "old-pass-123", "new-pass-456"); err != nil {
		t.Fatalf("change password: %v", err)
	}

	_, err := env.svc.RefreshSession(context.Background(), other.RefreshToken, "", "")
	if !errors.Is(err, service.ErrSessionRevoked) {
		t.Fatalf("other session after password change = %v, want ErrSessionRevoked", err)
	}
	if _, err := env.svc.RefreshSession(context.Background(), current.RefreshToken, "", ""); err != nil {
		t.Fatalf("current session must survive password change: %v", err)
	}
}
