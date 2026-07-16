package password_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/kyangconn/music-online-go/internal/pkg/password"
)

func TestValidateNewPasswordBoundaries(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		raw  string
		want error
	}{
		{name: "seven characters", raw: "1234567", want: password.ErrPasswordTooShort},
		{name: "eight Unicode characters", raw: "密码密码密码密码"},
		{name: "72 ASCII bytes", raw: strings.Repeat("a", 72)},
		{name: "73 ASCII bytes", raw: strings.Repeat("a", 73), want: password.ErrPasswordTooLong},
		{name: "72 multibyte UTF-8 bytes", raw: strings.Repeat("密", 24)},
		{name: "75 multibyte UTF-8 bytes", raw: strings.Repeat("密", 25), want: password.ErrPasswordTooLong},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := password.ValidateNewPassword(tt.raw)
			if !errors.Is(err, tt.want) {
				t.Fatalf("ValidateNewPassword() error = %v, want %v", err, tt.want)
			}
		})
	}
}

func TestHashAndVerifyPassword(t *testing.T) {
	t.Parallel()

	raw := "密码password"
	hash, err := password.HashPassword(raw)
	if err != nil {
		t.Fatalf("HashPassword() error = %v", err)
	}

	valid, err := password.VerifyPassword(raw, hash)
	if err != nil || !valid {
		t.Fatalf("VerifyPassword() = %v, %v; want true, nil", valid, err)
	}
}

func TestVerifyPasswordTreatsOversizedInputAsMismatch(t *testing.T) {
	t.Parallel()

	hash, err := password.HashPassword("valid-password")
	if err != nil {
		t.Fatalf("HashPassword() error = %v", err)
	}

	valid, err := password.VerifyPassword(strings.Repeat("a", 73), hash)
	if err != nil || valid {
		t.Fatalf("VerifyPassword() = %v, %v; want false, nil", valid, err)
	}
}

func TestVerifyPasswordRejectsEmptyHash(t *testing.T) {
	t.Parallel()

	valid, err := password.VerifyPassword("valid-password", "")
	if valid || !errors.Is(err, password.ErrInvalidHash) {
		t.Fatalf("VerifyPassword() = %v, %v; want false, ErrInvalidHash", valid, err)
	}
}
