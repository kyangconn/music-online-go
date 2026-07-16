// Package config config_test.go - 配置验证测试
// 测试 JWT secret 校验逻辑在非 debug 模式下拒绝弱密钥
package config

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/spf13/viper"
)

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
		{"empty in debug is rejected", "", "debug", true},
		{"short in debug is rejected", "short-secret", "debug", true},
		{"default in debug is ok", "your-secret-key-change-in-production", "debug", false},
		{"strong in release", "0123456789abcdef0123456789abcdef", "release", false},
		{"strong in debug", "abcdef0123456789abcdef0123456789", "debug", false},
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

func TestNormalizeTrustedProxies(t *testing.T) {
	proxies, err := NormalizeTrustedProxies([]string{
		"127.0.0.1, 10.0.0.0/8",
		"2001:db8::/32",
		"127.0.0.1",
	})
	if err != nil {
		t.Fatalf("NormalizeTrustedProxies: %v", err)
	}
	want := []string{"127.0.0.1", "10.0.0.0/8", "2001:db8::/32"}
	if !reflect.DeepEqual(proxies, want) {
		t.Fatalf("proxies = %#v, want %#v", proxies, want)
	}
	if _, err := NormalizeTrustedProxies([]string{"proxy.internal"}); err == nil {
		t.Fatal("hostname should not be accepted as a trusted proxy")
	}
}

func TestNormalizeAllowedOrigins(t *testing.T) {
	origins, err := NormalizeAllowedOrigins([]string{
		"https://Music.Example.com/",
		"http://localhost:5173, https://music.example.com",
	})
	if err != nil {
		t.Fatalf("NormalizeAllowedOrigins: %v", err)
	}
	want := []string{"https://music.example.com", "http://localhost:5173"}
	if !reflect.DeepEqual(origins, want) {
		t.Fatalf("origins = %#v, want %#v", origins, want)
	}

	for _, invalid := range []string{"*", "music.example.com", "https://example.com/path"} {
		if _, err := NormalizeAllowedOrigins([]string{invalid}); err == nil {
			t.Errorf("NormalizeAllowedOrigins(%q) expected error", invalid)
		}
	}
}

func TestValidateMetricsConfig(t *testing.T) {
	if err := ValidateMetricsConfig(MetricsConfig{}); err != nil {
		t.Fatalf("disabled metrics should not require a token: %v", err)
	}
	if err := ValidateMetricsConfig(MetricsConfig{Enabled: true}); err == nil {
		t.Fatal("enabled metrics without a token should fail")
	}
	if err := ValidateMetricsConfig(MetricsConfig{Enabled: true, Token: "scrape-secret"}); err != nil {
		t.Fatalf("enabled metrics with a token: %v", err)
	}
}

func TestValidateDatabaseConfigFailsFastForIncompletePostgres(t *testing.T) {
	cfg := DatabaseConfig{
		Type:                         "postgres",
		Port:                         "5432",
		SSLMode:                      "prefer",
		LogLevel:                     "auto",
		ConnectTimeoutSeconds:        10,
		ConnectionMaxLifetimeMinutes: 60,
		ConnectionMaxIdleTimeMinutes: 10,
	}
	err := ValidateDatabaseConfig(cfg)
	if err == nil {
		t.Fatal("incomplete postgres config should fail")
	}
	for _, field := range []string{"database.host", "database.user", "database.name"} {
		if !strings.Contains(err.Error(), field) {
			t.Fatalf("error %q does not identify missing %s", err, field)
		}
	}

	cfg.Host = "postgres"
	cfg.User = "music"
	cfg.Name = "music"
	if err := ValidateDatabaseConfig(cfg); err != nil {
		t.Fatalf("complete postgres config: %v", err)
	}
}

func TestValidateRateLimitConfig(t *testing.T) {
	if err := ValidateRateLimitConfig(RateLimitConfig{}); err != nil {
		t.Fatalf("disabled rate limit should accept zero values: %v", err)
	}
	if err := ValidateRateLimitConfig(RateLimitConfig{Enabled: true}); err == nil {
		t.Fatal("enabled rate limit should reject zero values")
	}
	if err := ValidateRateLimitConfig(RateLimitConfig{
		Enabled:                 true,
		GlobalRequestsPerSecond: 20,
		GlobalBurst:             50,
		AuthRequestsPerSecond:   1,
		AuthBurst:               5,
	}); err != nil {
		t.Fatalf("valid rate limit: %v", err)
	}
}

func TestValidateLoggingConfig(t *testing.T) {
	valid := LoggingConfig{Level: "warn", MaxSizeMB: 50, MaxBackups: 3, MaxAgeDays: 28}
	if err := ValidateLoggingConfig(valid); err != nil {
		t.Fatalf("valid logging config: %v", err)
	}
	valid.Level = "verbose"
	if err := ValidateLoggingConfig(valid); err == nil {
		t.Fatal("unsupported logging level should fail")
	}
}

func TestLoadConfigEnvironmentOverridesFileAndDefaults(t *testing.T) {
	original := AppConfig
	t.Cleanup(func() { AppConfig = original })

	path := filepath.Join(t.TempDir(), "container-config.yaml")
	contents := []byte(`server:
  mode: release
  port: "8081"
database:
  type: sqlite
  path: from-file.db
jwt:
  secret: file-secret
`)
	if err := os.WriteFile(path, contents, 0600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	t.Setenv("MO_CONFIG_FILE", path)
	t.Setenv("SERVER_PORT", "9090")
	t.Setenv("SERVER_ALLOWED_ORIGINS", "https://one.example,https://two.example")
	t.Setenv("SERVER_TRUSTED_PROXIES", "127.0.0.1,10.0.0.0/8")
	t.Setenv("DATABASE_TYPE", "")
	t.Setenv("DATABASE_PATH", "")
	t.Setenv("DATABASE_PORT", "")
	t.Setenv("JWT_SECRET", "environment-secret-32-bytes-long!!")
	if err := LoadConfig(); err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}

	if AppConfig.Server.Port != "9090" {
		t.Fatalf("server port = %q, want environment override", AppConfig.Server.Port)
	}
	if AppConfig.Database.Path != "from-file.db" {
		t.Fatalf("database path = %q, want file value", AppConfig.Database.Path)
	}
	if AppConfig.Database.Port != "5432" {
		t.Fatalf("database port = %q, want code default", AppConfig.Database.Port)
	}
	if AppConfig.JWT.Secret != "environment-secret-32-bytes-long!!" {
		t.Fatalf("jwt secret did not use environment override")
	}
	if got, want := AppConfig.Server.AllowedOrigins, []string{"https://one.example", "https://two.example"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("allowed origins = %#v, want %#v", got, want)
	}
	if got, want := AppConfig.Server.TrustedProxies, []string{"127.0.0.1", "10.0.0.0/8"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("trusted proxies = %#v, want %#v", got, want)
	}
}

func TestLoadConfigReadsSensitiveValuesFromFiles(t *testing.T) {
	original := AppConfig
	t.Cleanup(func() { AppConfig = original })

	writeSecret := func(name, value string) string {
		t.Helper()
		path := filepath.Join(t.TempDir(), name)
		if err := os.WriteFile(path, []byte(value+"\r\n"), 0600); err != nil {
			t.Fatalf("write secret %s: %v", name, err)
		}
		return path
	}
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	contents := []byte(`server:
  mode: release
database:
  type: sqlite
  path: secrets-test.db
metrics:
  enabled: true
jwt:
  secret: yaml-value-must-be-overridden
`)
	if err := os.WriteFile(configPath, contents, 0600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	t.Setenv("MO_CONFIG_FILE", configPath)
	for _, envName := range []string{"JWT_SECRET", "DATABASE_PASSWORD", "METRICS_TOKEN", "ADMIN_BOOTSTRAP_PASSWORD"} {
		t.Setenv(envName, "")
	}
	t.Setenv("JWT_SECRET_FILE", writeSecret("jwt", "0123456789abcdef0123456789abcdef"))
	t.Setenv("DATABASE_PASSWORD_FILE", writeSecret("database", "database-secret"))
	t.Setenv("METRICS_TOKEN_FILE", writeSecret("metrics", "metrics-secret"))
	t.Setenv("ADMIN_BOOTSTRAP_PASSWORD_FILE", writeSecret("admin", "admin-secret"))

	if err := LoadConfig(); err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if AppConfig.JWT.Secret != "0123456789abcdef0123456789abcdef" ||
		AppConfig.Database.Password != "database-secret" ||
		AppConfig.Metrics.Token != "metrics-secret" ||
		AppConfig.AdminBootstrap.Password != "admin-secret" {
		t.Fatalf("secret file values were not loaded correctly: %+v", AppConfig)
	}
}

func TestSecretValueRejectsAmbiguousOrOversizedSources(t *testing.T) {
	viper.Reset()
	t.Cleanup(viper.Reset)
	viper.Set("jwt.secret", "yaml-secret")

	path := filepath.Join(t.TempDir(), "jwt")
	if err := os.WriteFile(path, []byte("file-secret"), 0600); err != nil {
		t.Fatalf("write secret: %v", err)
	}
	t.Setenv("JWT_SECRET", "direct-secret")
	t.Setenv("JWT_SECRET_FILE", path)
	if _, err := secretValue("jwt.secret", "JWT_SECRET"); err == nil || !strings.Contains(err.Error(), "cannot both be set") {
		t.Fatalf("ambiguous secret sources error = %v", err)
	}

	t.Setenv("JWT_SECRET", "")
	if err := os.WriteFile(path, make([]byte, maxSecretFileBytes+1), 0600); err != nil {
		t.Fatalf("write oversized secret: %v", err)
	}
	if _, err := secretValue("jwt.secret", "JWT_SECRET"); err == nil || !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("oversized secret error = %v", err)
	}
}

func TestLoadConfigUsesFirstSearchPathWithoutMergingLowerPriorityFile(t *testing.T) {
	original := AppConfig
	t.Cleanup(func() { AppConfig = original })

	root := t.TempDir()
	workDir := filepath.Join(root, "work")
	if err := os.MkdirAll(workDir, 0700); err != nil {
		t.Fatalf("create work directory: %v", err)
	}
	writeConfig := func(path, port, databasePath string) {
		t.Helper()
		contents := []byte("server:\n  port: \"" + port + "\"\ndatabase:\n  path: " + databasePath + "\njwt:\n  secret: search-order-test-secret-32-bytes\n")
		if err := os.WriteFile(path, contents, 0600); err != nil {
			t.Fatalf("write config %s: %v", path, err)
		}
	}
	writeConfig(filepath.Join(root, "config.yaml"), "8123", "higher-priority.db")
	writeConfig(filepath.Join(workDir, "config.yaml"), "8124", "lower-priority.db")

	t.Setenv("MO_CONFIG_FILE", "")
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(root, "empty-xdg"))
	t.Setenv("SERVER_PORT", "")
	t.Setenv("DATABASE_PATH", "")
	t.Setenv("JWT_SECRET", "")
	t.Chdir(workDir)

	if err := LoadConfig(); err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if AppConfig.Server.Port != "8123" || AppConfig.Database.Path != "higher-priority.db" {
		t.Fatalf("loaded port/path = %q/%q, want first ../ config only", AppConfig.Server.Port, AppConfig.Database.Path)
	}
}
