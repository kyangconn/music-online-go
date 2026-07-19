package mediatoken

import (
	"errors"
	"testing"
	"time"
)

func TestMediaTokenIsBoundToTrackKindAndExpiry(t *testing.T) {
	secret := "0123456789abcdef0123456789abcdef"
	now := time.Unix(1_700_000_000, 0)
	token := Issue(secret, 42, "stream", now.Add(time.Minute))

	if err := Validate(token, secret, 42, "stream", now); err != nil {
		t.Fatalf("valid token: %v", err)
	}
	for name, err := range map[string]error{
		"wrong track":  Validate(token, secret, 43, "stream", now),
		"wrong kind":   Validate(token, secret, 42, "cover", now),
		"expired":      Validate(token, secret, 42, "stream", now.Add(time.Minute)),
		"wrong secret": Validate(token, "different-secret", 42, "stream", now),
	} {
		if !errors.Is(err, ErrInvalidToken) {
			t.Errorf("%s error = %v, want ErrInvalidToken", name, err)
		}
	}
}
