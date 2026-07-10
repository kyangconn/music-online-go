package service_test

import (
	"context"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/kyangconn/music-online-go/internal/config"
	"github.com/kyangconn/music-online-go/internal/domain"
	"github.com/kyangconn/music-online-go/internal/repository"
	"github.com/kyangconn/music-online-go/internal/service"
	"gorm.io/gorm"
)

func newBootstrapUserService(t *testing.T) service.UserService {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&domain.User{}); err != nil {
		t.Fatalf("migrate users: %v", err)
	}
	config.AppConfig = &config.Config{
		JWT: config.JWTConfig{Secret: "test-secret", ExpireHour: 24},
	}

	return service.NewUserService(repository.NewUserRepository(db))
}

func TestBootstrapAdminCreatesAdmin(t *testing.T) {
	svc := newBootstrapUserService(t)
	ctx := context.Background()

	user, created, err := svc.BootstrapAdmin(ctx, service.BootstrapAdminRequest{
		Username: "admin",
		Email:    "admin@example.com",
		Password: "password123",
		FullName: "Administrator",
	})
	if err != nil {
		t.Fatalf("bootstrap admin: %v", err)
	}
	if !created {
		t.Fatal("expected bootstrap to create a user")
	}
	if user.Role != "admin" || !user.IsActive || !user.IsVerified {
		t.Fatalf("unexpected admin state: %+v", user)
	}

	login, err := svc.Login(ctx, &domain.LoginRequest{Username: "admin", Password: "password123"})
	if err != nil {
		t.Fatalf("login bootstrap admin: %v", err)
	}
	if login.User.Role != "admin" {
		t.Fatalf("login role = %q, want admin", login.User.Role)
	}
}

func TestBootstrapAdminPromotesExistingUserAndCanResetPassword(t *testing.T) {
	svc := newBootstrapUserService(t)
	ctx := context.Background()

	if _, err := svc.Register(ctx, &domain.RegisterRequest{
		Username: "owner",
		Email:    "owner@example.com",
		Password: "oldpass123",
	}); err != nil {
		t.Fatalf("register user: %v", err)
	}

	user, created, err := svc.BootstrapAdmin(ctx, service.BootstrapAdminRequest{
		Username:      "owner",
		Email:         "owner@example.com",
		Password:      "newpass123",
		FullName:      "Owner",
		ResetPassword: true,
	})
	if err != nil {
		t.Fatalf("bootstrap existing admin: %v", err)
	}
	if created {
		t.Fatal("expected existing user to be promoted, not recreated")
	}
	if user.Role != "admin" {
		t.Fatalf("role = %q, want admin", user.Role)
	}

	if _, err := svc.Login(ctx, &domain.LoginRequest{Username: "owner", Password: "oldpass123"}); err == nil {
		t.Fatal("old password should not work after reset")
	}
	if _, err := svc.Login(ctx, &domain.LoginRequest{Username: "owner", Password: "newpass123"}); err != nil {
		t.Fatalf("new password should work: %v", err)
	}
}
