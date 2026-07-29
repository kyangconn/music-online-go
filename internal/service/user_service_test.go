// Package service_test user_service_test.go - 用户服务 login 错误传播测试
// 用 stub repository 验证 Login 在各种错误场景下的行为：
//   - FindByUsername 返回非 NotFound 错误时不被掩蔽为 "invalid credentials"
//   - 用户名/邮箱查找回退逻辑
//   - 无效密码正确处理
//   - 有效凭据正常返回 token
package service_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kyangconn/music-online-go/internal/config"
	"github.com/kyangconn/music-online-go/internal/domain"
	"github.com/kyangconn/music-online-go/internal/pkg/password"
	"github.com/kyangconn/music-online-go/internal/repository"
	"github.com/kyangconn/music-online-go/internal/service"
)

// stubUserRepo 实现 repository.UserRepository，通过函数字段注入自定义行为。
// 未注入的方法返回安全默认值（nil 或 ErrUserNotFound）。
type stubUserRepo struct {
	createFn         func(ctx context.Context, user *domain.User) error
	findByUsernameFn func(ctx context.Context, username string) (*domain.User, error)
	findByEmailFn    func(ctx context.Context, email string) (*domain.User, error)
	findByIDFn       func(ctx context.Context, id uint) (*domain.User, error)
	updateFn         func(ctx context.Context, user *domain.User) error
	countAdminsFn    func(ctx context.Context) (int64, error)
	listMusicIDsFn   func(ctx context.Context, id uint) ([]uint, error)
	deleteFn         func(ctx context.Context, id uint) error
}

func (s *stubUserRepo) Create(ctx context.Context, user *domain.User) error {
	if s.createFn != nil {
		return s.createFn(ctx, user)
	}
	return nil
}

func (s *stubUserRepo) FindByID(ctx context.Context, id uint) (*domain.User, error) {
	if s.findByIDFn != nil {
		return s.findByIDFn(ctx, id)
	}
	return nil, repository.ErrUserNotFound
}

func (s *stubUserRepo) FindByUsername(_ context.Context, username string) (*domain.User, error) {
	if s.findByUsernameFn != nil {
		return s.findByUsernameFn(nil, username)
	}
	return nil, repository.ErrUserNotFound
}

func (s *stubUserRepo) FindByEmail(_ context.Context, email string) (*domain.User, error) {
	if s.findByEmailFn != nil {
		return s.findByEmailFn(nil, email)
	}
	return nil, repository.ErrUserNotFound
}

func (s *stubUserRepo) Update(ctx context.Context, user *domain.User) error {
	if s.updateFn != nil {
		return s.updateFn(ctx, user)
	}
	return nil
}

func (s *stubUserRepo) Delete(ctx context.Context, id uint) error {
	if s.deleteFn != nil {
		return s.deleteFn(ctx, id)
	}
	return nil
}

func (s *stubUserRepo) ListOwnedMusicIDs(ctx context.Context, id uint) ([]uint, error) {
	if s.listMusicIDsFn != nil {
		return s.listMusicIDsFn(ctx, id)
	}
	return nil, nil
}

func (s *stubUserRepo) ExistsByUsername(_ context.Context, _ string) (bool, error) { return false, nil }

func (s *stubUserRepo) ExistsByEmail(_ context.Context, _ string) (bool, error) { return false, nil }

func (s *stubUserRepo) List(_ context.Context, _, _ int) ([]*domain.User, int64, error) {
	return nil, 0, nil
}

func (s *stubUserRepo) UpdateStatus(_ context.Context, _ uint, _ bool) error { return nil }

func (s *stubUserRepo) UpdateRole(_ context.Context, _ uint, _ string) error { return nil }

func (s *stubUserRepo) Search(_ context.Context, _ string, _, _ int) ([]*domain.User, int64, error) {
	return nil, 0, nil
}

func (s *stubUserRepo) CountAdmins(ctx context.Context) (int64, error) {
	if s.countAdminsFn != nil {
		return s.countAdminsFn(ctx)
	}
	return 0, nil
}

func (s *stubUserRepo) CountAll(_ context.Context) (int64, error) { return 0, nil }

func (s *stubUserRepo) SetTOTPSecret(_ context.Context, _ uint, _ string) error { return nil }

func (s *stubUserRepo) SetTOTPEnabled(_ context.Context, _ uint, _ bool) error { return nil }

// assertLoginErrIs 检查 login 返回的错误是否匹配 want，未匹配时标记测试失败。
func assertLoginErrIs(t *testing.T, err error, want error) {
	t.Helper()
	if !errors.Is(err, want) {
		t.Fatalf("expected error %v, got %v", want, err)
	}
}

// makeActiveUser 构造一个带预哈希密码、可登录的活跃用户。
func makeActiveUser(t *testing.T, username, rawPassword string) *domain.User {
	t.Helper()
	hash, err := password.HashPassword(rawPassword)
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	return &domain.User{
		Username: username,
		Password: hash,
		IsActive: true,
	}
}

func init() {
	config.AppConfig = &config.Config{
		JWT: config.JWTConfig{Secret: "test-secret", ExpireHour: 1},
	}
}

// TestLoginFallbackToEmail 验证：按用户名找不到时，回退到邮箱查询可成功。
func TestLoginFallbackToEmail(t *testing.T) {
	user := makeActiveUser(t, "u", "pwd12345")
	repo := &stubUserRepo{
		findByUsernameFn: func(_ context.Context, _ string) (*domain.User, error) {
			return nil, repository.ErrUserNotFound
		},
		findByEmailFn: func(_ context.Context, _ string) (*domain.User, error) {
			return user, nil
		},
	}
	svc := service.NewUserService(repo, t.TempDir())

	resp, err := svc.Login(context.Background(), &domain.LoginRequest{Username: "u@example.com", Password: "pwd12345"})
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	if resp.Token == "" {
		t.Fatal("token should not be empty")
	}
}

// TestLoginDBErrorNotSwallowed 验证：FindByUsername 返回非 NotFound 错误时，
// Login 不会将其掩蔽为 "invalid credentials"，而是直接传播底层错误。
func TestLoginDBErrorNotSwallowed(t *testing.T) {
	dbErr := errors.New("database connection lost")
	repo := &stubUserRepo{
		findByUsernameFn: func(_ context.Context, _ string) (*domain.User, error) {
			return nil, dbErr
		},
	}
	svc := service.NewUserService(repo, t.TempDir())

	_, err := svc.Login(context.Background(), &domain.LoginRequest{Username: "someone", Password: "x"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if errors.Is(err, service.ErrInvalidCredentials) {
		t.Fatalf("DB error was masked as invalid credentials: %v", err)
	}
}

// TestLoginUserNotFound 验证：用户名和邮箱都找不到时，返回 ErrInvalidCredentials。
func TestLoginUserNotFound(t *testing.T) {
	repo := &stubUserRepo{
		findByUsernameFn: func(_ context.Context, _ string) (*domain.User, error) {
			return nil, repository.ErrUserNotFound
		},
		findByEmailFn: func(_ context.Context, _ string) (*domain.User, error) {
			return nil, repository.ErrUserNotFound
		},
	}
	svc := service.NewUserService(repo, t.TempDir())

	_, err := svc.Login(context.Background(), &domain.LoginRequest{Username: "ghost", Password: "x"})
	assertLoginErrIs(t, err, service.ErrInvalidCredentials)
}

// TestLoginWrongPassword 验证：用户名存在但密码错误时，返回 ErrInvalidCredentials。
func TestLoginWrongPassword(t *testing.T) {
	user := makeActiveUser(t, "alice", "correct1")
	repo := &stubUserRepo{
		findByUsernameFn: func(_ context.Context, _ string) (*domain.User, error) {
			return user, nil
		},
	}
	svc := service.NewUserService(repo, t.TempDir())

	_, err := svc.Login(context.Background(), &domain.LoginRequest{Username: "alice", Password: "wrong"})
	assertLoginErrIs(t, err, service.ErrInvalidCredentials)
}

func TestLoginOversizedPasswordIsInvalidCredentials(t *testing.T) {
	user := makeActiveUser(t, "alice", "correct-password")
	repo := &stubUserRepo{
		findByUsernameFn: func(_ context.Context, _ string) (*domain.User, error) {
			return user, nil
		},
	}
	svc := service.NewUserService(repo, t.TempDir())

	_, err := svc.Login(context.Background(), &domain.LoginRequest{
		Username: "alice",
		Password: strings.Repeat("a", 73),
	})
	assertLoginErrIs(t, err, service.ErrInvalidCredentials)
}

// TestLoginValidCredentials 验证：正确凭据登录成功，返回 token。
func TestLoginValidCredentials(t *testing.T) {
	user := makeActiveUser(t, "bob", "secret123")
	repo := &stubUserRepo{
		findByUsernameFn: func(_ context.Context, _ string) (*domain.User, error) {
			return user, nil
		},
	}
	svc := service.NewUserService(repo, t.TempDir())

	resp, err := svc.Login(context.Background(), &domain.LoginRequest{Username: "bob", Password: "secret123"})
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	if resp.Token == "" {
		t.Fatal("token should not be empty")
	}
	if resp.User.Username != "bob" {
		t.Fatalf("username = %q, want bob", resp.User.Username)
	}
}

func TestRegisterRejectsOversizedPasswordBeforeCreate(t *testing.T) {
	created := false
	repo := &stubUserRepo{createFn: func(_ context.Context, _ *domain.User) error {
		created = true
		return nil
	}}
	svc := service.NewUserService(repo, t.TempDir())

	_, err := svc.Register(context.Background(), &domain.RegisterRequest{
		Username: "too-long",
		Email:    "too-long@example.com",
		Password: strings.Repeat("密", 25),
	})

	if !errors.Is(err, password.ErrPasswordTooLong) {
		t.Fatalf("Register() error = %v, want ErrPasswordTooLong", err)
	}
	if created {
		t.Fatal("repository Create was called for an invalid password")
	}
}

func TestChangePasswordRejectsOversizedPasswordBeforeUpdate(t *testing.T) {
	user := makeActiveUser(t, "change-password", "current-password")
	updated := false
	repo := &stubUserRepo{
		findByIDFn: func(_ context.Context, _ uint) (*domain.User, error) { return user, nil },
		updateFn: func(_ context.Context, _ *domain.User) error {
			updated = true
			return nil
		},
	}
	svc := service.NewUserService(repo, t.TempDir())

	err := svc.ChangePassword(context.Background(), 1, "current-password", strings.Repeat("a", 73))

	if !errors.Is(err, password.ErrPasswordTooLong) {
		t.Fatalf("ChangePassword() error = %v, want ErrPasswordTooLong", err)
	}
	if updated {
		t.Fatal("repository Update was called for an invalid password")
	}
}

func TestDeleteUserRejectsIncorrectPassword(t *testing.T) {
	user := makeActiveUser(t, "delete-user", "correct-password")
	deleted := false
	repo := &stubUserRepo{
		findByIDFn: func(_ context.Context, _ uint) (*domain.User, error) { return user, nil },
		deleteFn: func(_ context.Context, _ uint) error {
			deleted = true
			return nil
		},
	}
	svc := service.NewUserService(repo, t.TempDir())

	err := svc.DeleteUser(context.Background(), 1, "wrong-password")

	assertLoginErrIs(t, err, service.ErrInvalidCredentials)
	if deleted {
		t.Fatal("repository delete was called for an invalid password")
	}
}

func TestDeleteUserProtectsLastActiveAdmin(t *testing.T) {
	user := makeActiveUser(t, "last-admin", "correct-password")
	user.Role = "admin"
	deleted := false
	repo := &stubUserRepo{
		findByIDFn:    func(_ context.Context, _ uint) (*domain.User, error) { return user, nil },
		countAdminsFn: func(_ context.Context) (int64, error) { return 1, nil },
		deleteFn: func(_ context.Context, _ uint) error {
			deleted = true
			return nil
		},
	}
	svc := service.NewUserService(repo, t.TempDir())

	err := svc.DeleteUser(context.Background(), 1, "correct-password")

	assertLoginErrIs(t, err, service.ErrLastActiveAdmin)
	if deleted {
		t.Fatal("repository delete was called for the last active admin")
	}
}

func TestDeleteUserCleansOwnedMusicDirectoriesAfterDatabaseDelete(t *testing.T) {
	uploadDir := t.TempDir()

	user := makeActiveUser(t, "delete-user", "correct-password")
	for _, id := range []uint{10, 11} {
		dir := filepath.Join(uploadDir, fmt.Sprintf("%d", id))
		if err := os.MkdirAll(dir, 0700); err != nil {
			t.Fatalf("create upload directory: %v", err)
		}
		if err := os.WriteFile(filepath.Join(dir, "audio.mp3"), []byte("audio"), 0600); err != nil {
			t.Fatalf("write upload: %v", err)
		}
	}
	deleted := false
	repo := &stubUserRepo{
		findByIDFn:     func(_ context.Context, _ uint) (*domain.User, error) { return user, nil },
		listMusicIDsFn: func(_ context.Context, _ uint) ([]uint, error) { return []uint{10, 11}, nil },
		deleteFn: func(_ context.Context, _ uint) error {
			deleted = true
			return nil
		},
	}
	svc := service.NewUserService(repo, uploadDir)

	if err := svc.DeleteUser(context.Background(), 1, "correct-password"); err != nil {
		t.Fatalf("delete user: %v", err)
	}
	if !deleted {
		t.Fatal("repository delete was not called")
	}
	for _, id := range []uint{10, 11} {
		if _, err := os.Stat(filepath.Join(uploadDir, fmt.Sprintf("%d", id))); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("upload directory %d still exists or stat failed: %v", id, err)
		}
	}
}
