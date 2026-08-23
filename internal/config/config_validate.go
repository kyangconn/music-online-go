// Package config config_validate.go - 配置校验
package config

import (
	"errors"
	"fmt"
	"math"
	"net/mail"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/kyangconn/music-online-go/internal/pkg/password"
)

func ValidateMetricsConfig(cfg MetricsConfig) error {
	if cfg.Enabled && strings.TrimSpace(cfg.Token) == "" {
		return errors.New("metrics token is required when metrics are enabled")
	}
	return nil
}

func ValidateAdminBootstrapConfig(cfg AdminBootstrapConfig) error {
	if !cfg.Enabled {
		return nil
	}
	usernameLength := utf8.RuneCountInString(cfg.Username)
	if usernameLength < 3 || usernameLength > 50 {
		return errors.New("enabled admin.bootstrap requires a username between 3 and 50 characters")
	}
	if len(cfg.Email) > 255 {
		return errors.New("admin.bootstrap.email exceeds 255 bytes")
	}
	address, err := mail.ParseAddress(cfg.Email)
	if err != nil || address.Address != cfg.Email {
		return errors.New("enabled admin.bootstrap requires a valid email address")
	}
	if utf8.RuneCountInString(cfg.FullName) > 255 {
		return errors.New("admin.bootstrap.full_name exceeds 255 characters")
	}
	if err := password.ValidateNewPassword(cfg.Password); err != nil {
		return fmt.Errorf("admin.bootstrap.password: %w", err)
	}
	return nil
}

func Validate(cfg *Config) error {
	if cfg == nil {
		return errors.New("config is nil")
	}
	if err := ValidateServerConfig(cfg.Server); err != nil {
		return err
	}
	if err := ValidateDatabaseConfig(cfg.Database); err != nil {
		return err
	}
	if err := ValidateJWTSecret(cfg.JWT.Secret, cfg.Server.Mode); err != nil {
		return err
	}
	if cfg.JWT.AccessTokenTTLMinutes <= 0 {
		return errors.New("jwt.access_token_ttl_minutes must be greater than zero")
	}
	if cfg.JWT.RefreshTokenTTLDays <= 0 {
		return errors.New("jwt.refresh_token_ttl_days must be greater than zero")
	}
	if err := ValidateMetricsConfig(cfg.Metrics); err != nil {
		return err
	}
	if err := ValidateAdminBootstrapConfig(cfg.AdminBootstrap); err != nil {
		return err
	}
	if err := ValidateAccessConfig(cfg.Access); err != nil {
		return err
	}
	if err := ValidateLibraryConfig(cfg.Library); err != nil {
		return err
	}
	if err := ValidateClassificationConfig(cfg.Classification); err != nil {
		return err
	}
	if err := ValidateMusicBeeConfig(cfg.Integrations.MusicBee); err != nil {
		return err
	}
	if err := ValidateRateLimitConfig(cfg.RateLimit); err != nil {
		return err
	}
	return ValidateLoggingConfig(cfg.Logging)
}

func ValidateClassificationConfig(cfg ClassificationConfig) error {
	if !cfg.Enabled {
		if cfg.AnalyzeOnUpload {
			return errors.New("classification.analyze_on_upload requires classification.enabled")
		}
		return nil
	}
	if cfg.AutoThreshold <= 0 || cfg.AutoThreshold > 1 {
		return errors.New("classification.auto_threshold must be greater than 0 and at most 1")
	}
	if cfg.ReviewMargin < 0 || cfg.ReviewMargin > 1 {
		return errors.New("classification.review_margin must be between 0 and 1")
	}
	for name, value := range map[string]float64{
		"classification.weights.calm_flow":     cfg.CalmFlowWeight,
		"classification.weights.kinetic_pulse": cfg.KineticPulseWeight,
		"classification.weights.cosmic_drift":  cfg.CosmicDriftWeight,
		"classification.weights.bass_impact":   cfg.BassImpactWeight,
	} {
		if value <= 0 || value > 2 {
			return fmt.Errorf("%s must be greater than 0 and at most 2", name)
		}
	}
	if cfg.AnalyzeOnUpload && cfg.Analyzer.Mode != "http" {
		return errors.New("classification.analyze_on_upload requires classification.analyzer.mode=http")
	}
	return ValidateAnalyzerConfig(cfg.Analyzer)
}

func ValidateAnalyzerConfig(cfg AnalyzerConfig) error {
	switch cfg.Mode {
	case "", "disabled":
		return nil
	case "http":
	default:
		return fmt.Errorf("classification.analyzer.mode must be disabled or http, got %q", cfg.Mode)
	}
	endpoint, err := url.Parse(cfg.Endpoint)
	if err != nil || (endpoint.Scheme != "http" && endpoint.Scheme != "https") || endpoint.Host == "" ||
		endpoint.User != nil || endpoint.Fragment != "" {
		return errors.New("classification.analyzer.endpoint must be an absolute http(s) URL without credentials or fragment")
	}
	if len([]byte(cfg.Token)) < 32 {
		return errors.New("classification.analyzer.token must contain at least 32 bytes in http mode")
	}
	for name, value := range map[string]string{
		"classification.analyzer.id":            cfg.ID,
		"classification.analyzer.version":       cfg.Version,
		"classification.analyzer.model_version": cfg.ModelVersion,
	} {
		if value == "" || len(value) > 100 {
			return fmt.Errorf("%s must contain 1 to 100 characters in http mode", name)
		}
	}
	if cfg.TimeoutSeconds <= 0 || cfg.TimeoutSeconds > 3600 {
		return errors.New("classification.analyzer.timeout_seconds must be between 1 and 3600")
	}
	if cfg.Concurrency <= 0 || cfg.Concurrency > 8 {
		return errors.New("classification.analyzer.concurrency must be between 1 and 8")
	}
	if cfg.QueueLimit <= 0 || cfg.QueueLimit > 100000 {
		return errors.New("classification.analyzer.queue_limit must be between 1 and 100000")
	}
	if cfg.MaxFileSizeMB <= 0 || cfg.MaxDurationSeconds <= 0 {
		return errors.New("classification analyzer file-size and duration limits must be greater than zero")
	}
	if err := validateMegabytesFits("classification.analyzer.max_file_size_mb", cfg.MaxFileSizeMB); err != nil {
		return err
	}
	if err := validateDurationFits("classification.analyzer.max_duration_seconds", cfg.MaxDurationSeconds, time.Second); err != nil {
		return err
	}
	if cfg.RetryMaxAttempts <= 0 || cfg.RetryMaxAttempts > 20 {
		return errors.New("classification.analyzer.retry_max_attempts must be between 1 and 20")
	}
	if cfg.RetryInitialSeconds <= 0 || cfg.RetryMaxSeconds < cfg.RetryInitialSeconds {
		return errors.New("classification analyzer retry delays must be positive and max must not be smaller than initial")
	}
	if err := validateDurationFits("classification.analyzer.timeout_seconds", cfg.TimeoutSeconds, time.Second); err != nil {
		return err
	}
	if err := validateDurationFits("classification.analyzer.retry_max_seconds", cfg.RetryMaxSeconds, time.Second); err != nil {
		return err
	}
	return nil
}

func ValidateLibraryConfig(cfg LibraryConfig) error {
	if cfg.HealthCheckIntervalSeconds < 0 {
		return errors.New("library.health_check_interval_seconds cannot be negative")
	}
	if err := validateDurationFits("library.health_check_interval_seconds", cfg.HealthCheckIntervalSeconds, time.Second); err != nil {
		return err
	}
	if cfg.Scanner.MaxFileSizeMB <= 0 {
		return errors.New("library.scanner.max_file_size_mb must be greater than zero")
	}
	if err := validateMegabytesFits("library.scanner.max_file_size_mb", cfg.Scanner.MaxFileSizeMB); err != nil {
		return err
	}
	if cfg.Scanner.MaxTagSizeMB <= 0 || cfg.Scanner.MaxTagSizeMB > 64 {
		return errors.New("library.scanner.max_tag_size_mb must be between 1 and 64")
	}
	if cfg.Scanner.MinFileAgeSeconds < 0 {
		return errors.New("library.scanner.min_file_age_seconds cannot be negative")
	}
	if err := validateDurationFits("library.scanner.min_file_age_seconds", cfg.Scanner.MinFileAgeSeconds, time.Second); err != nil {
		return err
	}
	if cfg.Scanner.HashRecheckHours < 0 {
		return errors.New("library.scanner.hash_recheck_hours cannot be negative")
	}
	if err := validateDurationFits("library.scanner.hash_recheck_hours", cfg.Scanner.HashRecheckHours, time.Hour); err != nil {
		return err
	}
	if cfg.Scanner.RetryMaxAttempts <= 0 {
		return errors.New("library.scanner.retry_max_attempts must be greater than zero")
	}
	if cfg.Scanner.RetryInitialSeconds <= 0 {
		return errors.New("library.scanner.retry_initial_seconds must be greater than zero")
	}
	if err := validateDurationFits("library.scanner.retry_initial_seconds", cfg.Scanner.RetryInitialSeconds, time.Second); err != nil {
		return err
	}
	if cfg.Scanner.RetryMaxSeconds < cfg.Scanner.RetryInitialSeconds {
		return errors.New("library.scanner.retry_max_seconds must be greater than or equal to retry_initial_seconds")
	}
	if err := validateDurationFits("library.scanner.retry_max_seconds", cfg.Scanner.RetryMaxSeconds, time.Second); err != nil {
		return err
	}
	return nil
}

func ValidateAccessConfig(cfg AccessConfig) error {
	switch cfg.LibraryMode {
	case LibraryAccessPublic, LibraryAccessAuthenticated:
	default:
		return fmt.Errorf("access.library_mode must be public or authenticated, got %q", cfg.LibraryMode)
	}
	switch cfg.RegistrationMode {
	case RegistrationOpen, RegistrationAdmin:
	default:
		return fmt.Errorf("access.registration_mode must be open or admin, got %q", cfg.RegistrationMode)
	}
	if cfg.MediaURLTTLMinutes <= 0 {
		return errors.New("access.media_url_ttl_minutes must be greater than zero")
	}
	if err := validateDurationFits("access.media_url_ttl_minutes", cfg.MediaURLTTLMinutes, time.Minute); err != nil {
		return err
	}
	return nil
}

func ValidateMusicBeeConfig(cfg MusicBeeConfig) error {
	hasToken := strings.TrimSpace(cfg.SubmitToken) != ""
	hasUsername := strings.TrimSpace(cfg.SubmitUsername) != ""
	if hasToken != hasUsername {
		return errors.New("integrations.musicbee.submit_token and submit_username must be configured together")
	}
	if hasToken && len([]byte(cfg.SubmitToken)) < 32 {
		return errors.New("integrations.musicbee.submit_token must contain at least 32 bytes")
	}
	return nil
}

func ValidateServerConfig(cfg ServerConfig) error {
	port, err := strconv.Atoi(cfg.Port)
	if err != nil || port < 1 || port > 65535 {
		return fmt.Errorf("server.port must be an integer between 1 and 65535, got %q", cfg.Port)
	}
	switch cfg.Mode {
	case "debug", "release", "test":
	default:
		return fmt.Errorf("server.mode must be debug, release, or test, got %q", cfg.Mode)
	}
	for name, value := range map[string]int{
		"server.read_header_timeout": cfg.ReadHeaderTimeout,
		"server.read_timeout":        cfg.ReadTimeout,
		"server.write_timeout":       cfg.WriteTimeout,
		"server.idle_timeout":        cfg.IdleTimeout,
	} {
		if value < 0 {
			return fmt.Errorf("%s cannot be negative", name)
		}
		if err := validateDurationFits(name, value, time.Second); err != nil {
			return err
		}
	}
	if cfg.ShutdownTimeout <= 0 {
		return errors.New("server.shutdown_timeout must be greater than zero")
	}
	if err := validateDurationFits("server.shutdown_timeout", cfg.ShutdownTimeout, time.Second); err != nil {
		return err
	}
	if cfg.ReadinessTimeout <= 0 {
		return errors.New("server.readiness_timeout must be greater than zero")
	}
	if err := validateDurationFits("server.readiness_timeout", cfg.ReadinessTimeout, time.Second); err != nil {
		return err
	}
	if strings.TrimSpace(cfg.UploadDir) == "" {
		return errors.New("server.upload_dir cannot be empty")
	}
	if cfg.MaxJSONBodySizeMB <= 0 {
		return errors.New("server.max_json_body_size_mb must be greater than zero")
	}
	if err := validateMegabytesFits("server.max_json_body_size_mb", cfg.MaxJSONBodySizeMB); err != nil {
		return err
	}
	if cfg.MaxAudioSizeMB <= 0 {
		return errors.New("server.max_audio_size_mb must be greater than zero")
	}
	if err := validateMegabytesFits("server.max_audio_size_mb", cfg.MaxAudioSizeMB); err != nil {
		return err
	}
	if cfg.MaxCoverSizeMB <= 0 {
		return errors.New("server.max_cover_size_mb must be greater than zero")
	}
	if err := validateMegabytesFits("server.max_cover_size_mb", cfg.MaxCoverSizeMB); err != nil {
		return err
	}
	audioBytes := int64(cfg.MaxAudioSizeMB) * (1 << 20)
	coverBytes := int64(cfg.MaxCoverSizeMB) * (1 << 20)
	if audioBytes > maxSignedInt64-coverBytes-(1<<20) {
		return errors.New("server audio and cover limits are too large to form a safe multipart request limit")
	}
	return nil
}

func ValidateDatabaseConfig(cfg DatabaseConfig) error {
	switch cfg.LogLevel {
	case "auto", "silent", "error", "warn", "info":
	default:
		return fmt.Errorf("database.log_level must be auto, silent, error, warn, or info, got %q", cfg.LogLevel)
	}
	switch cfg.Type {
	case "sqlite":
		if strings.TrimSpace(cfg.Path) == "" {
			return errors.New("database.path is required for sqlite")
		}
	case "postgres":
		missing := make([]string, 0, 3)
		if strings.TrimSpace(cfg.Host) == "" {
			missing = append(missing, "database.host")
		}
		if strings.TrimSpace(cfg.User) == "" {
			missing = append(missing, "database.user")
		}
		if strings.TrimSpace(cfg.Name) == "" {
			missing = append(missing, "database.name")
		}
		if len(missing) > 0 {
			return fmt.Errorf("postgres configuration is incomplete; missing %s", strings.Join(missing, ", "))
		}
		port, err := strconv.Atoi(cfg.Port)
		if err != nil || port < 1 || port > 65535 {
			return fmt.Errorf("database.port must be an integer between 1 and 65535, got %q", cfg.Port)
		}
		validSSLModes := map[string]struct{}{
			"disable": {}, "allow": {}, "prefer": {}, "require": {}, "verify-ca": {}, "verify-full": {},
		}
		if _, ok := validSSLModes[cfg.SSLMode]; !ok {
			return fmt.Errorf("database.sslmode has unsupported value %q", cfg.SSLMode)
		}
	default:
		return fmt.Errorf("database.type must be sqlite or postgres, got %q", cfg.Type)
	}
	if cfg.ConnectTimeoutSeconds <= 0 {
		return errors.New("database.connect_timeout_seconds must be greater than zero")
	}
	for name, value := range map[string]int{
		"database.max_open_connections":             cfg.MaxOpenConnections,
		"database.max_idle_connections":             cfg.MaxIdleConnections,
		"database.connection_max_lifetime_minutes":  cfg.ConnectionMaxLifetimeMinutes,
		"database.connection_max_idle_time_minutes": cfg.ConnectionMaxIdleTimeMinutes,
	} {
		if value < 0 {
			return fmt.Errorf("%s cannot be negative", name)
		}
		if strings.Contains(name, "_minutes") {
			if err := validateDurationFits(name, value, time.Minute); err != nil {
				return err
			}
		}
	}
	if cfg.MaxOpenConnections > 0 && cfg.MaxIdleConnections > cfg.MaxOpenConnections {
		return errors.New("database.max_idle_connections cannot exceed database.max_open_connections")
	}
	return nil
}

func ValidateRateLimitConfig(cfg RateLimitConfig) error {
	if !cfg.Enabled {
		return nil
	}
	if !finitePositive(cfg.GlobalRequestsPerSecond) || cfg.GlobalBurst <= 0 {
		return errors.New("enabled rate_limit requires positive global_requests_per_second and global_burst")
	}
	if !finitePositive(cfg.AuthRequestsPerSecond) || cfg.AuthBurst <= 0 {
		return errors.New("enabled rate_limit requires positive auth_requests_per_second and auth_burst")
	}
	return nil
}

func finitePositive(value float64) bool {
	return value > 0 && !math.IsInf(value, 0) && !math.IsNaN(value)
}

func ValidateLoggingConfig(cfg LoggingConfig) error {
	switch cfg.Level {
	case "debug", "info", "warn", "error":
	default:
		return fmt.Errorf("logging.level must be debug, info, warn, or error, got %q", cfg.Level)
	}
	if cfg.MaxSizeMB <= 0 {
		return errors.New("logging.max_size_mb must be greater than zero")
	}
	if err := validateMegabytesFits("logging.max_size_mb", cfg.MaxSizeMB); err != nil {
		return err
	}
	if cfg.MaxBackups < 0 {
		return errors.New("logging.max_backups cannot be negative")
	}
	if cfg.MaxAgeDays < 0 {
		return errors.New("logging.max_age_days cannot be negative")
	}
	return nil
}

var maxSignedInt64 = int64(^uint64(0) >> 1)

func validateDurationFits(name string, value int, unit time.Duration) error {
	if value > 0 && int64(value) > maxSignedInt64/int64(unit) {
		return fmt.Errorf("%s is too large", name)
	}
	return nil
}

func validateMegabytesFits(name string, value int) error {
	if value > 0 && int64(value) > maxSignedInt64/(1<<20) {
		return fmt.Errorf("%s is too large", name)
	}
	return nil
}

// ValidateJWTSecret checks that the JWT secret is strong enough for non-debug modes.
// Debug mode may use the documented development placeholder, but never an empty
// signing key. Non-debug modes require an explicitly configured secret.
func ValidateJWTSecret(secret, mode string) error {
	if strings.TrimSpace(secret) == "" {
		return errors.New("jwt.secret is required")
	}
	if len([]byte(secret)) < 32 {
		return errors.New("jwt.secret must contain at least 32 bytes")
	}
	if mode != "debug" && secret == "your-secret-key-change-in-production" {
		return errors.New("weak JWT secret rejected: must be set to a strong random value in non-debug mode")
	}
	return nil
}
