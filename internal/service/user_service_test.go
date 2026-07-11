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
	findByUsernameFn func(ctx context.Context, username string) (*domain.User, error)
	findByEmailFn    func(ctx context.Context, email string) (*domain.User, error)
}

func (s *stubUserRepo) Create(_ context.Context, _ *domain.User) error { return nil }

func (s *stubUserRepo) FindByID(_ context.Context, _ uint) (*domain.User, error) {
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

func (s *stubUserRepo) Update(_ context.Context, _ *domain.User) error { return nil }

func (s *stubUserRepo) Delete(_ context.Context, _ uint) error { return nil }

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

func (s *stubUserRepo) CountAdmins(_ context.Context) (int64, error) { return 0, nil }

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
	user := makeActiveUser(t, "u", "pwd")
	repo := &stubUserRepo{
		findByUsernameFn: func(_ context.Context, _ string) (*domain.User, error) {
			return nil, repository.ErrUserNotFound
		},
		findByEmailFn: func(_ context.Context, _ string) (*domain.User, error) {
			return user, nil
		},
	}
	svc := service.NewUserService(repo)

	resp, err := svc.Login(context.Background(), &domain.LoginRequest{Username: "u@example.com", Password: "pwd"})
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
	svc := service.NewUserService(repo)

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
	svc := service.NewUserService(repo)

	_, err := svc.Login(context.Background(), &domain.LoginRequest{Username: "ghost", Password: "x"})
	assertLoginErrIs(t, err, service.ErrInvalidCredentials)
}

// TestLoginWrongPassword 验证：用户名存在但密码错误时，返回 ErrInvalidCredentials。
func TestLoginWrongPassword(t *testing.T) {
	user := makeActiveUser(t, "alice", "correct")
	repo := &stubUserRepo{
		findByUsernameFn: func(_ context.Context, _ string) (*domain.User, error) {
			return user, nil
		},
	}
	svc := service.NewUserService(repo)

	_, err := svc.Login(context.Background(), &domain.LoginRequest{Username: "alice", Password: "wrong"})
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
	svc := service.NewUserService(repo)

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
