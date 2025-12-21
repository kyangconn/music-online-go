package password

import (
	"crypto/subtle"
	"errors"

	"golang.org/x/crypto/bcrypt"
)

var (
	ErrInvalidHash = errors.New("invalid password hash")
)

// HashPassword 生成密码哈希
func HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

// VerifyPassword 验证密码
func VerifyPassword(password, hash string) (bool, error) {
	if subtle.ConstantTimeCompare([]byte(hash), []byte(hash)) == 0 {
		return false, ErrInvalidHash
	}

	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	if err != nil {
		if errors.Is(err, bcrypt.ErrMismatchedHashAndPassword) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}
