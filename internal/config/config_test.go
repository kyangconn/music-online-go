// Package config config_test.go - 配置验证测试
// 测试 JWT secret 校验逻辑在非 debug 模式下拒绝弱密钥
package config

import "testing"

func TestValidateJWTSecret(t *testing.T) {
	tests := []struct {
		name    string
		secret  string
		mode    string
		wantErr bool
	}{
		{"empty in release", "", "release", true},
		{"default in release", "your-secret-key-change-in-production", "release", true},
		{"default in test mode", "your-secret-key-change-in-production", "test", true},
		{"empty in debug is ok", "", "debug", false},
		{"default in debug is ok", "your-secret-key-change-in-production", "debug", false},
		{"strong in release", "super-secret-random-key", "release", false},
		{"strong in debug", "another-key", "debug", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateJWTSecret(tt.secret, tt.mode)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateJWTSecret(%q, %q) error = %v, wantErr = %v", tt.secret, tt.mode, err, tt.wantErr)
			}
		})
	}
}
