// Package service user_service.go - 用户服务层
// 包含用户注册、登录、信息更新、密码修改、TOTP 及可撤销会话管理等业务逻辑
package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/kyangconn/music-online-go/internal/config"
	"github.com/kyangconn/music-online-go/internal/domain"
	"github.com/kyangconn/music-online-go/internal/pkg/jwt"
	"github.com/kyangconn/music-online-go/internal/pkg/password"
	"github.com/kyangconn/music-online-go/internal/repository"
	"github.com/pquerna/otp/totp"
)

var (
	ErrInvalidCredentials   = errors.New("invalid credentials")
	ErrAccountInactive      = errors.New("account is inactive")
	ErrTOTPCodeRequired     = errors.New("totp code required")
	ErrInvalidTOTPCode      = errors.New("invalid totp code")
	ErrOldPasswordIncorrect = errors.New("old password is incorrect")
	ErrBootstrapAdminConfig = errors.New("bootstrap admin config is invalid")
	ErrLastActiveAdmin      = errors.New("cannot delete the last active admin")
	ErrInvalidRefreshToken  = errors.New("invalid refresh token")
	ErrSessionRevoked       = errors.New("session has been revoked")
	ErrSessionExpired       = errors.New("session has expired")
	// ErrConcurrentRefresh is returned when a refresh raced with another one
	// using the same token. The caller should retry with the rotated cookie.
	ErrConcurrentRefresh = errors.New("refresh raced with a concurrent rotation")
)

// refreshTokenBytes is the entropy of an opaque refresh token (32 bytes).
const refreshTokenBytes = 32

// concurrentRefreshGrace is the window in which a stale-but-unrevoked session
// is treated as a benign concurrent rotation (e.g. two browser tabs sharing
// the cookie) instead of a replayed/stolen token.
const concurrentRefreshGrace = 30 * time.Second

// UserService defines user business logic operations.
type UserService interface {
	Register(ctx context.Context, req *domain.RegisterRequest) (*domain.UserResponse, error)
	Login(ctx context.Context, req *domain.LoginRequest) (*domain.LoginResponse, error)
	RefreshSession(ctx context.Context, refreshToken, userAgent, ipAddress string) (*domain.LoginResponse, error)
	LogoutSession(ctx context.Context, userID, sessionID uint) error
	LogoutByRefreshToken(ctx context.Context, refreshToken string) error
	LogoutAllSessions(ctx context.Context, userID uint) error
	GetUserByID(ctx context.Context, id uint) (*domain.UserResponse, error)
	GetUserByUsername(ctx context.Context, username string) (*domain.UserResponse, error)
	UpdateUser(ctx context.Context, id uint, req *domain.UpdateUserRequest) (*domain.UserResponse, error)
	DeleteUser(ctx context.Context, id uint, currentPassword string) error
	ChangePassword(ctx context.Context, userID, sessionID uint, oldPassword, newPassword string) error
	VerifyUser(ctx context.Context, id uint) error
	ListUsers(ctx context.Context, page, pageSize int) ([]*domain.UserResponse, int64, error)
	// Admin methods
	UpdateUserStatus(ctx context.Context, id uint, isActive bool) error
	UpdateUserRole(ctx context.Context, id uint, role string) error
	SearchUsers(ctx context.Context, query string, page, pageSize int) ([]*domain.UserResponse, int64, error)
	CountAll(ctx context.Context) (int64, error)
	CountAdmins(ctx context.Context) (int64, error)
	CreateUserByAdmin(ctx context.Context, req *domain.AdminCreateUserRequest) (*domain.UserResponse, error)
	// TOTP
	SetupTOTP(ctx context.Context, userID uint) (*domain.TOTPSetupResponse, error)
	EnableTOTP(ctx context.Context, userID uint, code string) error
	DisableTOTP(ctx context.Context, userID uint, code string) error
	BootstrapAdmin(ctx context.Context, req BootstrapAdminRequest) (*domain.UserResponse, bool, error)
}

type userService struct {
	userRepo    repository.UserRepository
	sessionRepo repository.SessionRepository
	uploadDir   string
	jwtCfg      config.JWTConfig
}

type BootstrapAdminRequest struct {
	Username      string
	Email         string
	Password      string
	FullName      string
	ResetPassword bool
}

// dummyPasswordHash keeps failed login timing closer when the account does not exist.
const dummyPasswordHash = "$2a$10$OLIc7WDuS61Ho.Ezf91LNO9AOgRWT3WbAmBnvG2OrzkqLR9vOCnpC"

func NewUserService(
	userRepo repository.UserRepository,
	sessionRepo repository.SessionRepository,
	uploadDir string,
	jwtCfg config.JWTConfig,
) UserService {
	return &userService{userRepo: userRepo, sessionRepo: sessionRepo, uploadDir: uploadDir, jwtCfg: jwtCfg}
}

func (s *userService) Register(ctx context.Context, req *domain.RegisterRequest) (*domain.UserResponse, error) {
	// 哈希密码
	hashedPassword, err := password.HashPassword(req.Password)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	// 创建用户
	user := &domain.User{
		Username: req.Username,
		Email:    req.Email,
		Password: hashedPassword,
		FullName: req.FullName,
		Role:     "user",
	}

	if err := s.userRepo.Create(ctx, user); err != nil {
		return nil, err
	}

	return user.ToResponse(), nil
}

func (s *userService) CreateUserByAdmin(ctx context.Context, req *domain.AdminCreateUserRequest) (*domain.UserResponse, error) {
	hashedPassword, err := password.HashPassword(req.Password)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}
	user := &domain.User{
		Username:   req.Username,
		Email:      req.Email,
		Password:   hashedPassword,
		FullName:   req.FullName,
		Role:       req.Role,
		IsActive:   true,
		IsVerified: true,
	}
	if err := s.userRepo.Create(ctx, user); err != nil {
		return nil, err
	}
	return user.ToResponse(), nil
}

func (s *userService) BootstrapAdmin(ctx context.Context, req BootstrapAdminRequest) (*domain.UserResponse, bool, error) {
	if req.Username == "" || req.Email == "" {
		return nil, false, fmt.Errorf("%w: username and email are required", ErrBootstrapAdminConfig)
	}
	if err := password.ValidateNewPassword(req.Password); err != nil {
		return nil, false, fmt.Errorf("%w: %w", ErrBootstrapAdminConfig, err)
	}
	existing, err := s.userRepo.FindByUsername(ctx, req.Username)
	if err != nil {
		if !errors.Is(err, repository.ErrUserNotFound) {
			return nil, false, err
		}

		hashedPassword, hashErr := password.HashPassword(req.Password)
		if hashErr != nil {
			return nil, false, fmt.Errorf("failed to hash bootstrap admin password: %w", hashErr)
		}
		user := &domain.User{
			Username:   req.Username,
			Email:      req.Email,
			Password:   hashedPassword,
			FullName:   req.FullName,
			Role:       "admin",
			IsActive:   true,
			IsVerified: true,
		}
		if createErr := s.userRepo.Create(ctx, user); createErr != nil {
			return nil, false, createErr
		}
		return user.ToResponse(), true, nil
	}

	changed := false
	if existing.Role != "admin" {
		existing.Role = "admin"
		changed = true
	}
	if !existing.IsActive {
		existing.IsActive = true
		changed = true
	}
	if !existing.IsVerified {
		existing.IsVerified = true
		changed = true
	}
	if existing.FullName == "" {
		existing.FullName = req.FullName
		changed = true
	}
	if req.ResetPassword {
		hashedPassword, hashErr := password.HashPassword(req.Password)
		if hashErr != nil {
			return nil, false, fmt.Errorf("failed to hash bootstrap admin password: %w", hashErr)
		}
		existing.Password = hashedPassword
		changed = true
	}

	if changed {
		if err := s.userRepo.Update(ctx, existing); err != nil {
			return nil, false, err
		}
	}
	return existing.ToResponse(), false, nil
}

func (s *userService) Login(ctx context.Context, req *domain.LoginRequest) (*domain.LoginResponse, error) {
	// 查找用户（支持用户名或邮箱登录）
	var user *domain.User
	var err error

	// 尝试按用户名查找
	user, err = s.userRepo.FindByUsername(ctx, req.Username)
	if err != nil {
		// 只有明确"用户名不存在"时才回退到邮箱查询
		if !errors.Is(err, repository.ErrUserNotFound) {
			return nil, fmt.Errorf("failed to find user: %w", err)
		}
		// 如果按用户名找不到，尝试按邮箱查找
		user, err = s.userRepo.FindByEmail(ctx, req.Username)
		if err != nil {
			if _, verifyErr := password.VerifyPassword(req.Password, dummyPasswordHash); verifyErr != nil {
				return nil, fmt.Errorf("failed to verify password: %w", verifyErr)
			}
			return nil, ErrInvalidCredentials
		}
	}

	// 检查用户状态
	if !user.IsActive {
		return nil, ErrAccountInactive
	}

	// 验证密码
	valid, err := password.VerifyPassword(req.Password, user.Password)
	if err != nil {
		return nil, fmt.Errorf("failed to verify password: %w", err)
	}
	if !valid {
		return nil, ErrInvalidCredentials
	}

	if user.TOTPEnabled {
		if req.TOTPCode == "" {
			return nil, ErrTOTPCodeRequired
		}
		if !totp.Validate(req.TOTPCode, user.TOTPSecret) {
			return nil, ErrInvalidTOTPCode
		}
	}

	return s.createLoginSession(ctx, user, "", "")
}

// createLoginSession opens a new server-side session and mints a short-lived
// access token for it. The opaque refresh token is returned only through the
// LoginResponse (json:"-") so the handler can place it in an httpOnly cookie.
func (s *userService) createLoginSession(ctx context.Context, user *domain.User, userAgent, ipAddress string) (*domain.LoginResponse, error) {
	random, err := randomHex(refreshTokenBytes)
	if err != nil {
		return nil, fmt.Errorf("generate refresh token: %w", err)
	}

	now := time.Now()
	session := &domain.Session{
		UserID:      user.ID,
		RefreshHash: hashRefreshToken("pending-" + random),
		UserAgent:   truncate(userAgent, 256),
		IPAddress:   truncate(ipAddress, 64),
		LastUsedAt:  now,
		ExpiresAt:   now.Add(time.Duration(s.jwtCfg.RefreshTokenTTLDays) * 24 * time.Hour),
	}
	if err := s.sessionRepo.Create(ctx, session); err != nil {
		return nil, fmt.Errorf("create session: %w", err)
	}

	// 会话 ID 确定后把真正的 refresh token 绑定到该会话。绑定失败时
	// 清理刚创建的占位行，避免残留不可用的 pending 会话。
	refreshToken := refreshTokenForSession(session.ID, random)
	if _, err := s.sessionRepo.RotateIfMatch(ctx, session.ID, session.RefreshHash, hashRefreshToken(refreshToken), session.UserAgent, session.IPAddress, now); err != nil {
		_ = s.sessionRepo.DeleteByID(ctx, session.ID)
		return nil, fmt.Errorf("bind refresh token: %w", err)
	}

	accessToken, err := jwt.GenerateToken(user.ID, user.Username, user.Role, session.ID, s.jwtCfg.AccessTokenTTLMinutes, s.jwtCfg.Secret)
	if err != nil {
		return nil, fmt.Errorf("generate access token: %w", err)
	}
	// 顺带清理该用户已过期的会话，避免表无限增长。
	if err := s.sessionRepo.DeleteExpiredForUser(ctx, user.ID, now); err != nil {
		return nil, fmt.Errorf("clean expired sessions: %w", err)
	}

	return &domain.LoginResponse{
		User:         user.ToResponse(),
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    s.jwtCfg.AccessTokenTTLMinutes * 60,
	}, nil
}

// RefreshSession validates an opaque refresh token, rotates it to a fresh
// value and returns a new short-lived access token. A rotated token that is
// replayed outside the concurrency grace window revokes the whole session
// (reuse detection); within the window it is treated as a benign race and the
// caller is asked to retry with the new cookie.
func (s *userService) RefreshSession(ctx context.Context, refreshToken, userAgent, ipAddress string) (*domain.LoginResponse, error) {
	sessionID, _, ok := parseRefreshToken(refreshToken)
	if !ok {
		return nil, ErrInvalidRefreshToken
	}
	session, err := s.sessionRepo.FindByID(ctx, sessionID)
	if err != nil {
		if errors.Is(err, repository.ErrSessionNotFound) {
			return nil, ErrInvalidRefreshToken
		}
		return nil, err
	}

	now := time.Now()
	if session.RevokedAt != nil {
		return nil, ErrSessionRevoked
	}
	if now.After(session.ExpiresAt) {
		if err := s.sessionRepo.DeleteExpiredForUser(ctx, session.UserID, now); err != nil {
			return nil, err
		}
		return nil, ErrSessionExpired
	}

	user, err := s.userRepo.FindByID(ctx, session.UserID)
	if err != nil {
		if errors.Is(err, repository.ErrUserNotFound) {
			// 用户已删除：立即撤销其残留会话。
			_ = s.sessionRepo.DeleteByUserID(ctx, session.UserID)
			return nil, ErrSessionRevoked
		}
		return nil, err
	}
	if !user.IsActive {
		if err := s.sessionRepo.RevokeAllForUser(ctx, user.ID, now); err != nil {
			return nil, err
		}
		return nil, ErrSessionRevoked
	}

	newRefreshToken, err := randomRefreshTokenForSession(session.ID)
	if err != nil {
		return nil, fmt.Errorf("generate refresh token: %w", err)
	}
	rotated, err := s.sessionRepo.RotateIfMatch(ctx, session.ID, hashRefreshToken(refreshToken), hashRefreshToken(newRefreshToken), truncate(userAgent, 256), truncate(ipAddress, 64), now)
	if err != nil {
		return nil, err
	}
	if rotated == 0 {
		// 哈希不再匹配：并发轮换或重放。仅在宽限窗口内容忍并发。
		current, lookupErr := s.sessionRepo.FindByID(ctx, session.ID)
		if lookupErr != nil {
			// 数据库错误必须直接向上传播，不能误判为重放。
			if !errors.Is(lookupErr, repository.ErrSessionNotFound) {
				return nil, lookupErr
			}
			// 会话行已不存在：视为已失效。
			return nil, ErrSessionRevoked
		}
		if current.RevokedAt == nil && now.Sub(current.UpdatedAt) <= concurrentRefreshGrace {
			return nil, ErrConcurrentRefresh
		}
		_ = s.sessionRepo.Revoke(ctx, session.ID, now)
		return nil, ErrSessionRevoked
	}

	accessToken, err := jwt.GenerateToken(user.ID, user.Username, user.Role, session.ID, s.jwtCfg.AccessTokenTTLMinutes, s.jwtCfg.Secret)
	if err != nil {
		return nil, fmt.Errorf("generate access token: %w", err)
	}

	return &domain.LoginResponse{
		AccessToken:  accessToken,
		RefreshToken: newRefreshToken,
		ExpiresIn:    s.jwtCfg.AccessTokenTTLMinutes * 60,
	}, nil
}

// LogoutSession revokes the session identified by sessionID if it belongs to
// userID. It is safe to call for already revoked or missing sessions.
func (s *userService) LogoutSession(ctx context.Context, userID, sessionID uint) error {
	session, err := s.sessionRepo.FindByID(ctx, sessionID)
	if err != nil {
		if errors.Is(err, repository.ErrSessionNotFound) {
			return nil
		}
		return err
	}
	if session.UserID != userID {
		return nil
	}
	return s.sessionRepo.Revoke(ctx, sessionID, time.Now())
}

// LogoutByRefreshToken revokes the session owned by the given refresh token.
// Holding a valid refresh token is itself proof of session ownership, so this
// path lets clients sign out even when their access token has already expired
// (e.g. the logout request itself would be rejected by AuthMiddleware).
func (s *userService) LogoutByRefreshToken(ctx context.Context, refreshToken string) error {
	sessionID, _, ok := parseRefreshToken(refreshToken)
	if !ok {
		return nil
	}
	session, err := s.sessionRepo.FindByID(ctx, sessionID)
	if err != nil {
		if errors.Is(err, repository.ErrSessionNotFound) {
			return nil
		}
		return err
	}
	if session.RevokedAt != nil {
		return nil
	}
	return s.sessionRepo.Revoke(ctx, sessionID, time.Now())
}

// LogoutAllSessions revokes every session of the user.
func (s *userService) LogoutAllSessions(ctx context.Context, userID uint) error {
	return s.sessionRepo.RevokeAllForUser(ctx, userID, time.Now())
}

// randomRefreshTokenForSession returns an opaque refresh token bound to the
// session ID so replay detection can always locate the owning session row.
func randomRefreshTokenForSession(sessionID uint) (string, error) {
	random, err := randomHex(refreshTokenBytes)
	if err != nil {
		return "", err
	}
	return refreshTokenForSession(sessionID, random), nil
}

func refreshTokenForSession(sessionID uint, random string) string {
	return fmt.Sprintf("s%d.%s", sessionID, random)
}

// parseRefreshToken extracts the session ID from a bound refresh token.
func parseRefreshToken(token string) (sessionID uint, random string, ok bool) {
	if !strings.HasPrefix(token, "s") {
		return 0, "", false
	}
	dot := strings.IndexByte(token, '.')
	if dot <= 1 || dot == len(token)-1 {
		return 0, "", false
	}
	parsed, err := strconv.ParseUint(token[1:dot], 10, 32)
	if err != nil || parsed == 0 {
		return 0, "", false
	}
	return uint(parsed), token[dot+1:], true
}

func randomHex(byteCount int) (string, error) {
	raw := make([]byte, byteCount)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return hex.EncodeToString(raw), nil
}

// hashRefreshToken hashes a refresh token so plaintext values never touch the
// database.
func hashRefreshToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func truncate(value string, maxRunes int) string {
	runes := []rune(value)
	if len(runes) <= maxRunes {
		return value
	}
	return string(runes[:maxRunes])
}

func (s *userService) GetUserByID(ctx context.Context, id uint) (*domain.UserResponse, error) {
	user, err := s.userRepo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return user.ToResponse(), nil
}

func (s *userService) GetUserByUsername(ctx context.Context, username string) (*domain.UserResponse, error) {
	user, err := s.userRepo.FindByUsername(ctx, username)
	if err != nil {
		return nil, err
	}
	return user.ToResponse(), nil
}

func (s *userService) UpdateUser(ctx context.Context, id uint, req *domain.UpdateUserRequest) (*domain.UserResponse, error) {
	user, err := s.userRepo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	// 更新允许修改的字段
	if req.FullName != nil {
		user.FullName = *req.FullName
	}
	if req.Nickname != nil {
		user.Nickname = *req.Nickname
	}
	if req.AvatarURL != nil {
		user.AvatarURL = *req.AvatarURL
	}
	if req.Phone != nil {
		user.Phone = *req.Phone
	}
	if req.Bio != nil {
		user.Bio = *req.Bio
	}
	if req.Email != nil {
		user.Email = *req.Email
	}

	if err := s.userRepo.Update(ctx, user); err != nil {
		return nil, err
	}

	return user.ToResponse(), nil
}

func (s *userService) DeleteUser(ctx context.Context, id uint, currentPassword string) error {
	user, err := s.userRepo.FindByID(ctx, id)
	if err != nil {
		return err
	}

	valid, err := password.VerifyPassword(currentPassword, user.Password)
	if err != nil {
		return fmt.Errorf("failed to verify password: %w", err)
	}
	if !valid {
		return ErrInvalidCredentials
	}

	if user.Role == "admin" && user.IsActive {
		adminCount, err := s.userRepo.CountAdmins(ctx)
		if err != nil {
			return err
		}
		if adminCount <= 1 {
			return ErrLastActiveAdmin
		}
	}

	musicIDs, err := s.userRepo.ListOwnedMusicIDs(ctx, id)
	if err != nil {
		return err
	}
	if err := s.userRepo.Delete(ctx, id); err != nil {
		return err
	}
	if err := s.sessionRepo.DeleteByUserID(ctx, id); err != nil {
		return err
	}
	for _, musicID := range musicIDs {
		cleanupMusicUploadDirectoryAt(s.uploadDir, musicID)
	}
	return nil
}

func (s *userService) ChangePassword(ctx context.Context, userID, sessionID uint, oldPassword, newPassword string) error {
	user, err := s.userRepo.FindByID(ctx, userID)
	if err != nil {
		return err
	}

	// 验证旧密码
	valid, err := password.VerifyPassword(oldPassword, user.Password)
	if err != nil {
		return fmt.Errorf("failed to verify password: %w", err)
	}
	if !valid {
		return ErrOldPasswordIncorrect
	}

	// 哈希新密码
	hashedPassword, err := password.HashPassword(newPassword)
	if err != nil {
		return fmt.Errorf("failed to hash password: %w", err)
	}

	user.Password = hashedPassword
	if err := s.userRepo.Update(ctx, user); err != nil {
		return err
	}
	// 密码变更后撤销其他设备上的会话，当前会话（携带 sessionID）保留。
	if sessionID > 0 {
		return s.sessionRepo.RevokeAllExcept(ctx, userID, sessionID, time.Now())
	}
	return s.sessionRepo.RevokeAllForUser(ctx, userID, time.Now())
}

func (s *userService) VerifyUser(ctx context.Context, id uint) error {
	user, err := s.userRepo.FindByID(ctx, id)
	if err != nil {
		return err
	}

	user.IsVerified = true
	return s.userRepo.Update(ctx, user)
}

func (s *userService) ListUsers(ctx context.Context, page, pageSize int) ([]*domain.UserResponse, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	users, total, err := s.userRepo.List(ctx, page, pageSize)
	if err != nil {
		return nil, 0, err
	}

	responses := make([]*domain.UserResponse, len(users))
	for i, user := range users {
		responses[i] = user.ToResponse()
	}

	return responses, total, nil
}
