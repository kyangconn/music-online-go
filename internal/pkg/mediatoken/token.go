package mediatoken

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

const tokenVersion = "v1"

var ErrInvalidToken = errors.New("invalid media token")

func Issue(secret string, musicID uint, kind string, expiresAt time.Time) string {
	payload := fmt.Sprintf("%s.%d.%s.%d", tokenVersion, musicID, kind, expiresAt.Unix())
	signature := sign(secret, payload)
	return payload + "." + base64.RawURLEncoding.EncodeToString(signature)
}

func Validate(token, secret string, musicID uint, kind string, now time.Time) error {
	parts := strings.Split(token, ".")
	if len(parts) != 5 || parts[0] != tokenVersion || parts[2] != kind {
		return ErrInvalidToken
	}
	id, err := strconv.ParseUint(parts[1], 10, 64)
	if err != nil || id != uint64(musicID) {
		return ErrInvalidToken
	}
	expiresUnix, err := strconv.ParseInt(parts[3], 10, 64)
	if err != nil || !now.Before(time.Unix(expiresUnix, 0)) {
		return ErrInvalidToken
	}
	providedSignature, err := base64.RawURLEncoding.DecodeString(parts[4])
	if err != nil {
		return ErrInvalidToken
	}
	payload := strings.Join(parts[:4], ".")
	if !hmac.Equal(providedSignature, sign(secret, payload)) {
		return ErrInvalidToken
	}
	return nil
}

func sign(secret, payload string) []byte {
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(payload))
	return mac.Sum(nil)
}
