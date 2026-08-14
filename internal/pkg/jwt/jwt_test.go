package jwt

import (
	"errors"
	"testing"
	"time"

	jwtlib "github.com/golang-jwt/jwt/v4"
)

const testSecret = "0123456789abcdef0123456789abcdef"

func TestTokenContract(t *testing.T) {
	t.Run("generated token uses the accepted issuer and algorithm", func(t *testing.T) {
		token, err := GenerateToken(7, "listener", "user", 42, 15, testSecret)
		if err != nil {
			t.Fatalf("GenerateToken: %v", err)
		}
		claims, err := ParseToken(token, testSecret)
		if err != nil {
			t.Fatalf("ParseToken: %v", err)
		}
		if claims.UserID != 7 || claims.SessionID != 42 || claims.Issuer != tokenIssuer {
			t.Fatalf("unexpected claims: %+v", claims)
		}
	})

	t.Run("rejects another HMAC algorithm", func(t *testing.T) {
		token := signedTestToken(t, jwtlib.SigningMethodHS384, tokenIssuer, time.Now().Add(time.Hour))
		if _, err := ParseToken(token, testSecret); !errors.Is(err, ErrInvalidToken) {
			t.Fatalf("ParseToken error = %v, want ErrInvalidToken", err)
		}
	})

	t.Run("rejects a missing issuer", func(t *testing.T) {
		token := signedTestToken(t, jwtlib.SigningMethodHS256, "", time.Now().Add(time.Hour))
		if _, err := ParseToken(token, testSecret); !errors.Is(err, ErrInvalidToken) {
			t.Fatalf("ParseToken error = %v, want ErrInvalidToken", err)
		}
	})

	t.Run("preserves the expired-token error", func(t *testing.T) {
		token := signedTestToken(t, jwtlib.SigningMethodHS256, tokenIssuer, time.Now().Add(-time.Hour))
		if _, err := ParseToken(token, testSecret); !errors.Is(err, ErrExpiredToken) {
			t.Fatalf("ParseToken error = %v, want ErrExpiredToken", err)
		}
	})

	t.Run("expires after the configured ttl", func(t *testing.T) {
		token, err := GenerateToken(7, "listener", "user", 42, 0, testSecret)
		if err != nil {
			t.Fatalf("GenerateToken: %v", err)
		}
		if _, err := ParseToken(token, testSecret); !errors.Is(err, ErrExpiredToken) {
			t.Fatalf("ParseToken error = %v, want ErrExpiredToken for zero ttl", err)
		}
	})
}

func signedTestToken(t *testing.T, method jwtlib.SigningMethod, issuer string, expiresAt time.Time) string {
	t.Helper()
	token := jwtlib.NewWithClaims(method, Claims{RegisteredClaims: jwtlib.RegisteredClaims{
		ExpiresAt: jwtlib.NewNumericDate(expiresAt),
		Issuer:    issuer,
	}})
	signed, err := token.SignedString([]byte(testSecret))
	if err != nil {
		t.Fatalf("sign test token: %v", err)
	}
	return signed
}
