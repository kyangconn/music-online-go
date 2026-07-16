// Package password password.go - 密码管理
// 提供 bcrypt 密码哈希和验证功能
package password

import (
	"errors"
	"unicode/utf8"

	"golang.org/x/crypto/bcrypt"
)

var (
	ErrInvalidHash      = errors.New("invalid password hash")
	ErrPasswordTooShort = errors.New("password must be at least 8 Unicode characters")
	ErrPasswordTooLong  = errors.New("password must not exceed 72 UTF-8 bytes")
)

const (
	MinPasswordCharacters = 8
	MaxPasswordBytes      = 72
)

// ValidateNewPassword enforces the password contract used for registration,
// password changes, and bootstrap administrators. bcrypt only accepts up to
// 72 bytes, so the upper bound is measured after UTF-8 encoding rather than by
// character count.
func ValidateNewPassword(raw string) error {
	if utf8.RuneCountInString(raw) < MinPasswordCharacters {
		return ErrPasswordTooShort
	}
	if len([]byte(raw)) > MaxPasswordBytes {
		return ErrPasswordTooLong
	}
	return nil
}

// HashPassword 生成密码哈希
func HashPassword(raw string) (string, error) {
	if err := ValidateNewPassword(raw); err != nil {
		return "", err
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(raw), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

// VerifyPassword 验证密码
func VerifyPassword(raw, hash string) (bool, error) {
	if hash == "" {
		return false, ErrInvalidHash
	}
	// No bcrypt hash produced by this application can match a value over the
	// algorithm's 72-byte limit. Treat it as a normal mismatch instead of
	// leaking a bcrypt input error as an HTTP 500 response.
	if len([]byte(raw)) > MaxPasswordBytes {
		return false, nil
	}

	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(raw))
	if err != nil {
		if errors.Is(err, bcrypt.ErrMismatchedHashAndPassword) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}
